package agent

import (
	"context"
	"testing"

	"go-stock/backend/models"
)

func TestExecuteTaskHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewCronTaskApi().ExecuteTask(ctx, &models.CronTask{
		Name:     "canceled",
		TaskType: "custom",
	})
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}
