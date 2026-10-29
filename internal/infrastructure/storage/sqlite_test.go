package storage

import (
	"path/filepath"
	"tcpconnect/internal/domain"
	"testing"
)

func TestWorkspaceSurvivesReopen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "workspace.db")
	s, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	w, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	w.Services = append(w.Services, domain.Service{ID: "test", Name: "本地 TCP", Host: "127.0.0.1", Port: 9878})
	if e = s.Save(w); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	w, e = s.Load()
	if e != nil || len(w.Services) != 1 || w.Services[0].Name != "本地 TCP" {
		t.Fatalf("persistence failed: %v", e)
	}
}
