package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/covid-tracking/storage"
	"go.bryk.io/covid-tracking/utils"
	"go.bryk.io/x/amqp"
	"go.bryk.io/x/ccg/did"
	xlog "go.bryk.io/x/log"
)

// WorkerOptions provide the configuration settings available/required
// when creating a new API worker instance.
type WorkerOptions struct {
	// Storage mechanism connection string. If not supported by the API worker,
	// an error will be returned.
	Store string

	// Message broker connection string. Used by the API worker to receive
	// tasks and notifications.
	Broker string

	// Supported DID methods.
	Providers []*did.Provider

	// To handle output.
	Logger xlog.Logger
}

// Worker instances are responsible for asynchronously handling
// incoming tasks and notifications from the broker.
type Worker struct {
	name      string
	ctx       context.Context
	halt      context.CancelFunc
	sub       *amqp.Consumer
	log       xlog.Logger
	store     *storage.Handler
	providers []*did.Provider
}

// NewWorker returns a new worker instance.
func NewWorker(opts *WorkerOptions) (*Worker, error) {
	var err error
	seed := make([]byte, 4)
	_, _ = rand.Read(seed)

	// Get worker instance
	w := &Worker{
		name:      fmt.Sprintf("worker-%x", seed),
		providers: opts.Providers,
		log:       opts.Logger,
	}

	// Get storage handler
	w.store, err = storage.NewHandler(opts.Store)
	if err != nil {
		return nil, err
	}

	w.sub, err = amqp.NewConsumer(opts.Broker, []amqp.Option{
		amqp.WithTopology(utils.BrokerTopology()),
		amqp.WithName(w.name),
		amqp.WithLogger(w.log),
	}...)
	if err != nil {
		return nil, err
	}

	// Start event processing and return instance
	w.ctx, w.halt = context.WithCancel(context.Background())
	go w.eventLoop()
	return w, nil
}

// Close properly finish the worker execution.
func (w *Worker) Close() {
	w.halt()
	<-w.ctx.Done()
	_ = w.sub.Close()
	w.store.Close()
}

// Name returns the worker unique identifier.
func (w *Worker) Name() string {
	return w.name
}

// Process messages received from the "tasks" queue.
func (w *Worker) handleTasks(deliveries <-chan amqp.Delivery) {
	for msg := range deliveries {
		switch msg.Type {
		case "ct19.location_record":
			w.locationRecord(msg)
		case "ct19.new_did":
			w.publishDID(msg)
		default:
			w.log.WithFields(xlog.Fields{
				"kind":         msg.Type,
				"exchange":     msg.Exchange,
				"content-type": msg.ContentType,
				"id":           msg.MessageId,
				"size":         len(msg.Body),
			}).Warning("invalid message type")
		}
	}
}

// Validate and save location records.
func (w *Worker) locationRecord(msg amqp.Delivery) {
	defer func() {
		_ = msg.Ack(false)
	}()

	// Get author DID
	userDID, ok := msg.Headers["did"]
	if !ok {
		w.log.Error("record without DID")
		return
	}

	// Decode message contents
	req := &protov1.RecordRequest{}
	if err := req.Unmarshal(msg.Body); err != nil {
		w.log.Error("invalid record contents")
		return
	}

	// Resolve DID document for the credential's subject
	id, err := utils.ResolveDID(userDID.(string), w.providers)
	if err != nil {
		w.log.Error("invalid DID")
		return
	}

	// Validate records
	var records []*protov1.LocationRecord
	for _, r := range req.Records {
		if validateRecord(id, r) {
			records = append(records, r)
		}
	}

	// Store valid records and return final result
	if err := w.store.LocationRecords(records); err != nil {
		w.log.WithField("error", err.Error()).Error("failed to save record")
		return
	}

	// Success message
	w.log.WithFields(xlog.Fields{
		"did":       userDID.(string),
		"timestamp": msg.Timestamp.Unix(),
	}).Info("location record processed")
}

// Publish a new DID instance.
func (w *Worker) publishDID(msg amqp.Delivery) {
	defer func() {
		_ = msg.Ack(false)
	}()

	// Decode DID document
	doc := did.Document{}
	if err := json.Unmarshal(msg.Body, &doc); err != nil {
		w.log.Warning("invalid message contents")
	}
	id, err := did.FromDocument(&doc)
	if err != nil {
		w.log.Warning("invalid message contents")
	}

	// Submit publish request
	go publishDID(id, 18, w.log)
}

// Internal event processing
func (w *Worker) eventLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.sub.Ready():
			deliveries, _, err := w.sub.Subscribe(amqp.SubscribeOptions{Queue: "tasks"})
			if err != nil {
				w.log.Warning("failed to open tasks subscription")
			}
			go w.handleTasks(deliveries)
		}
	}
}
