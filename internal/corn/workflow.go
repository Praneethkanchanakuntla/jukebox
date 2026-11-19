package corn

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

func DeleteRooms(ctx workflow.Context) (int64, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result int64
	err := workflow.ExecuteActivity(ctx, "DeleteStaleRooms").Get(ctx, &result)
	if err != nil {
		return 0, err
	}
	return result, err
}
