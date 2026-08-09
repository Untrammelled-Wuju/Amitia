package notification

import (
	"context"
	"sync"
	"sync/atomic"
)

const (
	ObserverLevelMinimal int = iota
	ObserverLevelMethod
	ObserverLevelDiagnostic
)

type NotificationObserver interface {
	OnNotification(notification Notification)
}

type ObserverFunc func(notification Notification)

func (f ObserverFunc) OnNotification(notification Notification) {
	f(notification)
}

type CountingObserver struct {
	total    atomic.Int64
	byMethod sync.Map
}

func NewCountingObserver() *CountingObserver {
	return &CountingObserver{}
}

func (o *CountingObserver) OnNotification(n Notification) {
	o.total.Add(1)
	val, _ := o.byMethod.LoadOrStore(n.Method, &atomic.Int64{})
	counter := val.(*atomic.Int64)
	counter.Add(1)
}

func (o *CountingObserver) Total() int64 {
	return o.total.Load()
}

func (o *CountingObserver) Count(method string) int64 {
	val, ok := o.byMethod.Load(method)
	if !ok {
		return 0
	}
	return val.(*atomic.Int64).Load()
}

type ObserverSink struct {
	inner    NotificationSink
	observer NotificationObserver
}

func NewObserverSink(inner NotificationSink, observer NotificationObserver) *ObserverSink {
	return &ObserverSink{inner: inner, observer: observer}
}

func (s *ObserverSink) Publish(ctx context.Context, n Notification) error {
	if s.observer != nil {
		s.observer.OnNotification(n)
	}
	if s.inner != nil {
		return s.inner.Publish(ctx, n)
	}
	return nil
}
