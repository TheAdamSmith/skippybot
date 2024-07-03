package skippy

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
)

const (
	REMINDER_TAG    = "%s|REMINDER"
	MORNING_MSG_TAG = "%s|MORNING_MSG"
	DAILY_INTERVAL  = 1
)

type Scheduler struct {
	gocron.Scheduler
	jobSet map[string]bool
}

func NewScheduler() (*Scheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &Scheduler{
		Scheduler: scheduler,
		jobSet:    make(map[string]bool),
	}, nil
}

func (s *Scheduler) AddReminderJob(channelID string, duration time.Duration, jobFunc interface{}) error {
	tag := MakeReminderTag(channelID)
	_, err := s.NewJob(gocron.OneTimeJob(
		gocron.OneTimeJobStartDateTime(time.Now().Add(duration)),
	),
		gocron.NewTask(jobFunc),
		gocron.WithTags(tag),
	)
	if err != nil {
		return err
	}
	s.jobSet[tag] = true
	return nil
}

func (s *Scheduler) CancelReminderJob(channelID string) {
	tag := MakeReminderTag(channelID)
	s.RemoveByTags(tag)
	delete(s.jobSet, tag)
}

func (s *Scheduler) HasReminderJob(channelID string) bool {
	_, ok := s.jobSet[MakeMorningMsgTag(channelID)]
	return ok
}

func (s *Scheduler) AddMorningMsgJob(
	channelID string,
	atTime time.Time,
	jobFunc interface{},
) error {
	tag := MakeMorningMsgTag(channelID)
	_, err := s.NewJob(
		gocron.DailyJob(
			DAILY_INTERVAL,
			gocron.NewAtTimes(
				gocron.NewAtTime(
					uint(atTime.Hour()),
					uint(atTime.Minute()),
					uint(atTime.Second()),
				),
			),
		),
		gocron.NewTask(jobFunc),
		gocron.WithTags(tag),
	)
	if err != nil {
		return err
	}
	s.jobSet[tag] = true

	return nil
}

func (s *Scheduler) CancelMorningMsgJob(channelID string) {
	tag := MakeMorningMsgTag(channelID)
	s.RemoveByTags(tag)
	delete(s.jobSet, tag)
}

func (s *Scheduler) HasMorningMsgJob(channelID string) bool {
	_, ok := s.jobSet[MakeMorningMsgTag(channelID)]
	return ok
}

func MakeReminderTag(channelID string) string {
	return fmt.Sprintf(REMINDER_TAG, channelID)
}

func MakeMorningMsgTag(channelID string) string {
	return fmt.Sprintf(MORNING_MSG_TAG, channelID)
}
