package application_test

import (
	"path/filepath"
	"sync/atomic"
	"tcpconnect/internal/application"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/infrastructure/runtime"
	"tcpconnect/internal/infrastructure/storage"
	"testing"
	"time"
)

func TestInvalidSaveDoesNotReplaceWorkspace(t *testing.T) {
	repo, e := storage.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if e != nil {
		t.Fatal(e)
	}
	app, e := application.New(repo, runtime.Factory{})
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	w := app.Snapshot(0).Workspace
	w.Services = append(w.Services, domain.Service{ID: "a", Name: "TCP", Protocol: "tcp", Host: "127.0.0.1", Port: 9878})
	if e = app.Save(w); e != nil {
		t.Fatal(e)
	}
	w.Services[0].Port = 0
	if e = app.Save(w); e == nil {
		t.Fatal("invalid port accepted")
	}
	if app.Snapshot(0).Workspace.Services[0].Port != 9878 {
		t.Fatal("failed save changed state")
	}
	data, e := app.Export()
	if e != nil {
		t.Fatal(e)
	}
	if e = app.Import(data); e != nil {
		t.Fatal(e)
	}
	if e = app.Import(`{"services":[{"id":"x"}]}`); e == nil {
		t.Fatal("invalid import accepted")
	}
}

type delayedFactory struct {
	started, release chan struct{}
	session          *fakeSession
}

func (f delayedFactory) Connect(domain.SessionOptions) (domain.Session, error) {
	close(f.started)
	<-f.release
	return f.session, nil
}

type fakeSession struct{ closed atomic.Bool }

func (s *fakeSession) Send(domain.Request) error { return nil }
func (s *fakeSession) Close()                    { s.closed.Store(true) }
func (s *fakeSession) State() domain.SessionState {
	return domain.SessionState{Connected: !s.closed.Load()}
}
func (s *fakeSession) StartRepeat(domain.Request, int) error { return nil }
func (s *fakeSession) StopTask(string)                       {}

func TestDisconnectCancelsLateConnection(t *testing.T) {
	repo, err := storage.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	f := delayedFactory{make(chan struct{}), make(chan struct{}), &fakeSession{}}
	app, err := application.New(repo, f)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	w := app.Snapshot(0).Workspace
	w.Services = []domain.Service{{ID: "tcp", Name: "TCP", Protocol: "tcp", Host: "127.0.0.1", Port: 9879}}
	if err = app.Save(w); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Connect("tcp") }()
	<-f.started
	app.Disconnect("tcp")
	close(f.release)
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("canceled connection succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("connection did not finish")
	}
	if !f.session.closed.Load() || len(app.Status(0).Sessions) != 0 {
		t.Fatal("late connection leaked")
	}
}

func TestClearRequestEventsPreservesOtherRequestsAndConnections(t *testing.T) {
	repo, err := storage.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	app, err := application.New(repo, runtime.Factory{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	for _, e := range []domain.Event{{ServiceID: "one", RequestID: "A", Kind: "message"}, {ServiceID: "one", RequestID: "B", Kind: "message"}, {ServiceID: "one", Kind: "message"}, {ServiceID: "two", RequestID: "A", Kind: "message"}} {
		app.Emit(e)
	}
	app.ClearRequestEvents("one", "A")
	remaining := app.Status(0).Events
	if len(remaining) != 3 {
		t.Fatalf("cleared unrelated history: %v", remaining)
	}
	app.ClearRequestEvents("one", "")
	remaining = app.Status(0).Events
	if len(remaining) != 2 || remaining[0].RequestID != "B" || remaining[1].ServiceID != "two" {
		t.Fatal("cleared unrelated session")
	}
	app.Emit(domain.Event{ServiceID: "one", RequestID: "A", Kind: "message"})
	if len(app.Status(4).Events) != 1 {
		t.Fatal("event cursor reset after clear")
	}
}
