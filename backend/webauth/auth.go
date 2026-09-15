package webauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/events"
)

const (
	CookieName     = "gostock_session"
	SessionTTL     = 30 * 24 * time.Hour
	MinUsernameLen = 3
	MaxUsernameLen = 32
	MinPasswordLen = 6
	MaxPasswordLen = 100
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserDisabled       = errors.New("账号已禁用")
	ErrUserExists         = errors.New("用户名已存在")
	ErrRegisterClosed     = errors.New("未开放注册，请联系管理员创建账号")
	ErrInvalidSetupToken  = errors.New("需要有效的初始化密钥")
	ErrInvalidUsername    = errors.New("用户名为 3-32 位字母、数字或下划线")
	ErrInvalidPassword    = errors.New("密码长度为 6-100 位")
	ErrUnauthorized       = errors.New("未登录")
	ErrForbidden          = errors.New("需要管理员权限")
	ErrUserNotFound       = errors.New("用户不存在")
)

type User struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Username     string    `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"isAdmin"`
	Disabled     bool      `json:"disabled"`
}

func (User) TableName() string { return "web_users" }

type Session struct {
	Token     string    `gorm:"primaryKey;size:64"`
	UserID    uint      `gorm:"index"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

func (Session) TableName() string { return "web_sessions" }

type PublicUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
	Disabled bool   `json:"disabled"`
}

var authDB *gorm.DB
var registerMu sync.Mutex

func Init(sqlitePath string) error {
	if sqlitePath == "" {
		sqlitePath = "data/auth.db"
	}
	gdb, err := db.Open(sqlitePath)
	if err != nil {
		return err
	}
	if err := gdb.AutoMigrate(&User{}, &Session{}); err != nil {
		return err
	}
	authDB = gdb
	_ = authDB.Where("expires_at < ?", time.Now()).Delete(&Session{})
	return nil
}

func HasUsers() bool {
	var n int64
	authDB.Model(&User{}).Count(&n)
	return n > 0
}

func UserCount() int64 {
	var n int64
	authDB.Model(&User{}).Count(&n)
	return n
}

func SetupSecret() string {
	return strings.TrimSpace(os.Getenv("WEB_SETUP_SECRET"))
}

func RequireSetupToken() bool {
	return !HasUsers() && SetupSecret() != ""
}

func setupTokenOK(got string) bool {
	want := SetupSecret()
	if want == "" {
		return false
	}
	got = strings.TrimSpace(got)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func AllowRegister() bool {
	if !HasUsers() {
		// 空库不允许公开抢注管理员，必须先配置 WEB_ADMIN_* 或 WEB_SETUP_SECRET。
		return SetupSecret() != ""
	}
	v := strings.TrimSpace(os.Getenv("WEB_ALLOW_REGISTER"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if len(username) < MinUsernameLen || len(username) > MaxUsernameLen {
		return ErrInvalidUsername
	}
	for _, r := range username {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return ErrInvalidUsername
		}
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLen || len(password) > MaxPasswordLen {
		return ErrInvalidPassword
	}
	return nil
}

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Public(u *User) PublicUser {
	if u == nil {
		return PublicUser{}
	}
	return PublicUser{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, Disabled: u.Disabled}
}

func CreateUser(username, password string, isAdmin bool) (*User, error) {
	username = strings.TrimSpace(username)
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	var n int64
	authDB.Model(&User{}).Where("username = ?", username).Count(&n)
	if n > 0 {
		return nil, ErrUserExists
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &User{Username: username, PasswordHash: hash, IsAdmin: isAdmin}
	if err := authDB.Create(u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

func Register(username, password, setupToken string) (*User, error) {
	registerMu.Lock()
	defer registerMu.Unlock()
	if !AllowRegister() {
		return nil, ErrRegisterClosed
	}
	first := !HasUsers()
	if first && !setupTokenOK(setupToken) {
		return nil, ErrInvalidSetupToken
	}
	u, err := CreateUser(username, password, first)
	if err != nil {
		return nil, err
	}
	if first {
		_ = MigrateLegacyIfNeeded(u.ID)
	}
	return u, nil
}

func Authenticate(username, password string) (*User, error) {
	username = strings.TrimSpace(username)
	u := &User{}
	err := authDB.Where("username = ?", username).First(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if u.Disabled {
		return nil, ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateSession(userID uint) (*Session, error) {
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	s := &Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(SessionTTL),
		CreatedAt: time.Now(),
	}
	if err := authDB.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

func DeleteSession(token string) {
	if token == "" {
		return
	}
	authDB.Where("token = ?", token).Delete(&Session{})
	events.Default.DisconnectSession(token)
}

func UserBySession(token string) (*User, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	s := &Session{}
	if err := authDB.Where("token = ?", token).First(s).Error; err != nil {
		return nil, ErrUnauthorized
	}
	if time.Now().After(s.ExpiresAt) {
		authDB.Delete(s)
		return nil, ErrUnauthorized
	}
	u := &User{}
	if err := authDB.First(u, s.UserID).Error; err != nil {
		return nil, ErrUnauthorized
	}
	if u.Disabled {
		return nil, ErrUserDisabled
	}
	return u, nil
}

func ListUsers() ([]User, error) {
	var users []User
	err := authDB.Order("id asc").Find(&users).Error
	return users, err
}

func SetDisabled(id uint, disabled bool) error {
	res := authDB.Model(&User{}).Where("id = ?", id).Update("disabled", disabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	if disabled {
		authDB.Where("user_id = ?", id).Delete(&Session{})
		events.Default.DisconnectUser(id)
	}
	return nil
}

func GetUser(id uint) (*User, error) {
	u := &User{}
	if err := authDB.First(u, id).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// BootstrapAdminFromEnv 若设置 WEB_ADMIN_USER / WEB_ADMIN_PASSWORD 则确保该管理员存在。
func BootstrapAdminFromEnv() (*User, error) {
	username := strings.TrimSpace(os.Getenv("WEB_ADMIN_USER"))
	password := os.Getenv("WEB_ADMIN_PASSWORD")
	if username == "" || password == "" {
		return nil, nil
	}
	u := &User{}
	err := authDB.Where("username = ?", username).First(u).Error
	if err == nil {
		if !u.IsAdmin {
			_ = authDB.Model(u).Update("is_admin", true)
			u.IsAdmin = true
		}
		_ = MigrateLegacyIfNeeded(u.ID)
		return u, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	u, err = CreateUser(username, password, true)
	if err != nil {
		return nil, err
	}
	_ = MigrateLegacyIfNeeded(u.ID)
	return u, nil
}

// EnsureProvisioned 空库时必须已预置管理员，或配置了首次注册用的 WEB_SETUP_SECRET。
func EnsureProvisioned() error {
	if HasUsers() {
		return nil
	}
	if SetupSecret() != "" {
		return nil
	}
	return errors.New("未配置初始管理员：请设置 WEB_ADMIN_USER 和 WEB_ADMIN_PASSWORD，或设置 WEB_SETUP_SECRET 供首次注册使用")
}
