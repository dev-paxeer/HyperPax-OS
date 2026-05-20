// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package pubsub

import (
	"sync"
	"sync/atomic"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/pkg/errors"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

// subscriberBufferSize is the capacity of each subscriber's event channel.
const subscriberBufferSize = 64

type UnsubscribeFunc func()

type EventBus interface {
	AddTopic(name string, src <-chan coretypes.ResultEvent) error
	RemoveTopic(name string)
	Subscribe(name string) (<-chan coretypes.ResultEvent, UnsubscribeFunc, error)
	Topics() []string
}

type memEventBus struct {
	topics          map[string]<-chan coretypes.ResultEvent
	topicsMux       *sync.RWMutex
	subscribers     map[string]map[uint64]chan<- coretypes.ResultEvent
	subscribersMux  *sync.RWMutex
	currentUniqueID uint64
	droppedEvents   uint64 // atomic counter for dropped events
	logger          log.Logger
}

func NewEventBus(logger log.Logger) EventBus {
	return &memEventBus{
		topics:         make(map[string]<-chan coretypes.ResultEvent),
		topicsMux:      new(sync.RWMutex),
		subscribers:    make(map[string]map[uint64]chan<- coretypes.ResultEvent),
		subscribersMux: new(sync.RWMutex),
		logger:         logger,
	}
}

func (m *memEventBus) GenUniqueID() uint64 {
	return atomic.AddUint64(&m.currentUniqueID, 1)
}

func (m *memEventBus) Topics() (topics []string) {
	m.topicsMux.RLock()
	defer m.topicsMux.RUnlock()

	topics = make([]string, 0, len(m.topics))
	for topicName := range m.topics {
		topics = append(topics, topicName)
	}

	return topics
}

func (m *memEventBus) AddTopic(name string, src <-chan coretypes.ResultEvent) error {
	m.topicsMux.RLock()
	_, ok := m.topics[name]
	m.topicsMux.RUnlock()

	if ok {
		return errors.New("topic already registered")
	}

	m.topicsMux.Lock()
	m.topics[name] = src
	m.topicsMux.Unlock()

	go m.publishTopic(name, src)

	return nil
}

func (m *memEventBus) RemoveTopic(name string) {
	m.topicsMux.Lock()
	delete(m.topics, name)
	m.topicsMux.Unlock()
}

func (m *memEventBus) Subscribe(name string) (<-chan coretypes.ResultEvent, UnsubscribeFunc, error) {
	m.topicsMux.RLock()
	_, ok := m.topics[name]
	m.topicsMux.RUnlock()

	if !ok {
		return nil, nil, errors.Errorf("topic not found: %s", name)
	}

	ch := make(chan coretypes.ResultEvent, subscriberBufferSize)
	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	id := m.GenUniqueID()
	if _, ok := m.subscribers[name]; !ok {
		m.subscribers[name] = make(map[uint64]chan<- coretypes.ResultEvent)
	}
	m.subscribers[name][id] = ch

	unsubscribe := func() {
		m.subscribersMux.Lock()
		defer m.subscribersMux.Unlock()
		delete(m.subscribers[name], id)
	}

	return ch, unsubscribe, nil
}

func (m *memEventBus) publishTopic(name string, src <-chan coretypes.ResultEvent) {
	for {
		msg, ok := <-src
		if !ok {
			m.closeAllSubscribers(name)
			m.topicsMux.Lock()
			delete(m.topics, name)
			m.topicsMux.Unlock()
			return
		}
		m.publishAllSubscribers(name, msg)
	}
}

func (m *memEventBus) closeAllSubscribers(name string) {
	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	subscribers := m.subscribers[name]
	delete(m.subscribers, name)
	// #nosec G705
	for _, sub := range subscribers {
		close(sub)
	}
}

func (m *memEventBus) publishAllSubscribers(name string, msg coretypes.ResultEvent) {
	m.subscribersMux.RLock()
	defer m.subscribersMux.RUnlock()
	subscribers := m.subscribers[name]
	// #nosec G705
	for _, sub := range subscribers {
		select {
		case sub <- msg:
		default:
			total := atomic.AddUint64(&m.droppedEvents, 1)
			if m.logger != nil {
				m.logger.Debug("dropped event for slow subscriber", "topic", name, "total_dropped", total)
			}
		}
	}
}

// DroppedEvents returns the total number of events dropped due to slow subscribers.
func (m *memEventBus) DroppedEvents() uint64 {
	return atomic.LoadUint64(&m.droppedEvents)
}
