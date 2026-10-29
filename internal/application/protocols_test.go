package application_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tcpconnect/internal/application"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"tcpconnect/internal/infrastructure/runtime"
	"tcpconnect/internal/infrastructure/storage"
)

func TestImportedProtocolCreatesPersistentTemplates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.db")
	repo, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app, err := application.New(repo, runtime.Factory{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	xml, err := os.ReadFile("../../assets/fixspec/FIX44.xml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := fix.ParseDictionary(xml)
	if err != nil {
		t.Fatal(err)
	}
	allTypes := make([]string, 0, len(d.Messages))
	for _, m := range d.Messages {
		allTypes = append(allTypes, m.Type)
	}
	id, err := app.ImportProtocol("CUSTOM44.xml", string(xml), allTypes)
	if err != nil {
		t.Fatal(err)
	}
	v := app.Snapshot(0).Workspace
	if len(v.Services) != 1 || v.Services[0].Name != "CUSTOM44" || len(v.Dictionaries) != 1 || len(v.Requests) != 181 {
		t.Fatalf("wrong imported counts: %d/%d/%d", len(v.Services), len(v.Dictionaries), len(v.Requests))
	}
	types := map[string]bool{}
	for _, r := range v.Requests {
		if r.ServiceID != id || types[r.MessageType] {
			t.Fatal("duplicate or wrongly linked template")
		}
		types[r.MessageType] = true
		if r.MessageType == "A" && r.Values["HeartBtInt"] != "30" {
			t.Fatal("missing logon defaults")
		}
		if r.MessageType == "D" && r.Name != "NewOrderSingle (D)" {
			t.Fatal("wrong order template")
		}
	}
	// Filling templates must preserve customized cases and add only a removed type.
	for i, r := range v.Requests {
		if r.MessageType == "A" {
			v.Requests[i].Name = "My logon"
			v.Requests[i].PreScript = "return data;"
		}
	}
	kept := []domain.Request{}
	for _, r := range v.Requests {
		if r.MessageType != "D" {
			kept = append(kept, r)
		}
	}
	v.Requests = kept
	if err = app.Save(v); err != nil {
		t.Fatal(err)
	}
	if count, err := app.GenerateMessages(id, []string{"D"}); err != nil || count != 1 {
		t.Fatalf("fill: %d %v", count, err)
	}
	if count, err := app.GenerateMessages(id, []string{"D"}); err != nil || count != 0 {
		t.Fatalf("duplicate fill: %d %v", count, err)
	}
	for _, r := range app.Snapshot(0).Workspace.Requests {
		if r.MessageType == "A" && (r.Name != "My logon" || r.PreScript != "return data;") {
			t.Fatal("customized case overwritten")
		}
	}
	before, _ := app.Export()
	if _, err = app.CreateMessage(id, "UNKNOWN"); err == nil {
		t.Fatal("unknown type created")
	}
	after, _ := app.Export()
	if before != after {
		t.Fatal("failed create mutated workspace")
	}
	r, err := app.CreateMessage(id, "D")
	if err != nil || r.Name != "NewOrderSingle (D)" {
		t.Fatalf("create: %v %v", r.Name, err)
	}
	svc := v.Services[0]
	svc.ID = "second"
	svc.Name = "Second"
	if count, err := app.SaveService(svc, allTypes); err != nil || count != 181 {
		t.Fatalf("new service templates: %d %v", count, err)
	}
	svc.Port = 9877
	if count, err := app.SaveService(svc, nil); err != nil || count != 0 {
		t.Fatalf("edit service regenerated: %d %v", count, err)
	}
	app.Close()
	reopened, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.Load()
	if err != nil || len(saved.Requests) != 363 {
		t.Fatalf("templates not persisted: %d %v", len(saved.Requests), err)
	}
}

type failingProtocolRepo struct{ workspace domain.Workspace }

func (r *failingProtocolRepo) Load() (domain.Workspace, error) { return r.workspace, nil }
func (r *failingProtocolRepo) Save(domain.Workspace) error     { return errors.New("disk unavailable") }
func (r *failingProtocolRepo) Close() error                    { return nil }

func TestProtocolImportFailureIsAtomic(t *testing.T) {
	app, err := application.New(&failingProtocolRepo{}, runtime.Factory{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	xml := `<fix type="FIX" major="4" minor="4"><messages><message name="Custom" msgtype="U"><field name="Value" required="Y"/></message></messages><fields><field number="5000" name="Value" type="STRING"/></fields></fix>`
	if _, err = app.ImportProtocol("custom.xml", xml, []string{"U"}); err == nil {
		t.Fatal("failed storage accepted")
	}
	v := app.Snapshot(0).Workspace
	if len(v.Services)+len(v.Requests)+len(v.Dictionaries) != 0 {
		t.Fatal("partial import was retained")
	}
}

func TestSelectedMessagesOnlyAndInvalidSelectionsAreAtomic(t *testing.T) {
	repo, err := storage.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	app, err := application.New(repo, runtime.Factory{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	xml, err := os.ReadFile("../../assets/fixspec/FIX44.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, types := range [][]string{nil, {}, {"A", "UNKNOWN"}} {
		if _, err = app.ImportProtocol("selected.xml", string(xml), types); err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
	if v := app.Snapshot(0).Workspace; len(v.Services)+len(v.Dictionaries)+len(v.Requests) != 0 {
		t.Fatal("invalid import left partial data")
	}
	id, err := app.ImportProtocol("selected.xml", string(xml), []string{"A", "D", "A"})
	if err != nil {
		t.Fatal(err)
	}
	v := app.Snapshot(0).Workspace
	if len(v.Requests) != 2 {
		t.Fatalf("selection created %d templates, want 2", len(v.Requests))
	}
	for _, r := range v.Requests {
		if r.MessageType != "A" && r.MessageType != "D" {
			t.Fatalf("unselected type %s", r.MessageType)
		}
	}
	d, err := app.Dictionary(v.Services[0].DictionaryID)
	if err != nil || len(d.Messages) != 181 {
		t.Fatal("selected import truncated the protocol dictionary")
	}
	svc := v.Services[0]
	svc.ID = "selected-service"
	if _, err = app.SaveService(svc, nil); err == nil {
		t.Fatal("new FIX service accepted no selection")
	}
	if count, err := app.SaveService(svc, []string{"0"}); err != nil || count != 1 {
		t.Fatalf("new service selected templates: %d %v", count, err)
	}
	before, _ := app.Export()
	for _, types := range [][]string{nil, {"0", "UNKNOWN"}} {
		if _, err = app.GenerateMessages(id, types); err == nil {
			t.Fatal("invalid supplement accepted")
		}
	}
	after, _ := app.Export()
	if before != after {
		t.Fatal("invalid supplement changed saved records")
	}
	if count, err := app.GenerateMessages(id, []string{"0", "0", "A"}); err != nil || count != 1 {
		t.Fatalf("selected supplement: %d %v", count, err)
	}
	if count, err := app.GenerateMessages(id, []string{"0"}); err != nil || count != 0 {
		t.Fatalf("selected supplement duplicated: %d %v", count, err)
	}
	if len(app.Snapshot(0).Workspace.Requests) != 4 {
		t.Fatal("unselected templates added")
	}
}
