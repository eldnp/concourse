package db

import "time"

//counterfeiter:generate . Notifier
type Notifier interface {
	Notify() <-chan struct{}
	Close() error
}

func newConditionNotifier(bus NotificationsBus, channel string, cond func() (bool, error)) (Notifier, error) {
	notified, err := bus.Listen(channel, 1)
	if err != nil {
		return nil, err
	}

	notifier := &conditionNotifier{
		cond:    cond,
		bus:     bus,
		channel: channel,

		notified: notified,
		notify:   make(chan struct{}, 1),

		stop: make(chan struct{}),
	}

	go notifier.watch()

	return notifier, nil
}

type conditionNotifier struct {
	cond func() (bool, error)

	bus     NotificationsBus
	channel string

	notified chan Notification
	notify   chan struct{}

	stop chan struct{}
}

func (notifier *conditionNotifier) Notify() <-chan struct{} {
	return notifier.notify
}

func (notifier *conditionNotifier) Close() error {
	close(notifier.stop)
	return notifier.bus.Unlisten(notifier.channel, notifier.notified)
}

func (notifier *conditionNotifier) watch() {
	for {
		c, err := notifier.cond()
		if err != nil {
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-notifier.stop:
				return
			}
		}

		if c {
			notifier.sendNotification()
		}

	dance:
		for {
			select {
			case <-notifier.stop:
				return
			case n := <-notifier.notified:
				if n.Healthy {
					notifier.sendNotification()
				} else {
					break dance
				}
			}
		}
	}
}

func (notifier *conditionNotifier) sendNotification() {
	select {
	case notifier.notify <- struct{}{}:
	default:
	}
}

func newNoopNotifier() Notifier {
	return &noopNotifier{
		notify: make(chan struct{}, 1),
	}
}

type noopNotifier struct {
	notify chan struct{}
}

func (notifier *noopNotifier) Notify() <-chan struct{} {
	return notifier.notify
}

func (notifier *noopNotifier) Close() error {
	close(notifier.notify)
	return nil
}
