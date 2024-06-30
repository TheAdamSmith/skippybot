package skippy

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
)

const (
	REMINDER_TAG = "%s|REMINDER"
)

type Scheduler struct {
	gocron.Scheduler
}

func NewScheduler() (*Scheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &Scheduler{
		Scheduler: scheduler,
	}, nil
}

func (s *Scheduler) AddReminderJob(channelID string, duration time.Duration, jobFunc interface{}) error {
	_, err := s.NewJob(gocron.OneTimeJob(
		gocron.OneTimeJobStartDateTime(time.Now().Add(duration)),
	),
		gocron.NewTask(jobFunc),
		gocron.WithTags(MakeReminderTag(channelID)),
	)
	return err
}

func (s *Scheduler) CancelReminderJob(channelID string) {
	s.RemoveByTags(MakeReminderTag(channelID))
}

func MakeReminderTag(channelID string) string {
	return fmt.Sprintf(REMINDER_TAG, channelID)
}
