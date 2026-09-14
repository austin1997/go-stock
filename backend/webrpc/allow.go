package webrpc

// Allowed reports whether method may be invoked through the web HTTP RPC
// dispatcher. Only the Wails-exported frontend API (plus web file-import
// adapters) is permitted; other exported App methods stay internal.
func Allowed(method string) bool {
	_, ok := allowedMethods[method]
	return ok
}
