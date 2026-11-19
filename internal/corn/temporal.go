package corn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
)

const taskQueue = "jukebox"

func StartCleanupWorkflow(db *sql.DB) (client.Client, worker.Worker, error) {
	c, err := client.Dial(client.Options{HostPort: client.DefaultHostPort})
	if err != nil {
		return nil, nil, fmt.Errorf("connect to Temporal: %w", err)
	}

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(DeleteRooms)
	w.RegisterActivity(NewStore(db))

	if err := w.Start(); err != nil {
		c.Close()
		return nil, nil, fmt.Errorf("start Temporal cleanup worker: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := scheduleDeleteStaleRooms(ctx, c); err != nil {
		w.Stop()
		c.Close()
		return nil, nil, fmt.Errorf("create stale-room cleanup schedule: %w", err)
	}

	return c, w, nil
}

func scheduleDeleteStaleRooms(ctx context.Context, c client.Client) error {
	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: "delete-stale-rooms-schedule",
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{Every: 5 * time.Minute},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        "delete-stale-rooms",
			Workflow:  DeleteRooms,
			TaskQueue: taskQueue,
		},
		Overlap: enums.SCHEDULE_OVERLAP_POLICY_SKIP,
	})

	if errors.Is(err, temporal.ErrScheduleAlreadyRunning) {
		return nil
	}

	return err
}
