package domain

import (
	"tcpconnect/internal/domain/fix"
	"time"
)

type Service struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	TimeoutMS       int    `json:"timeoutMs"`
	FIXVersion      string `json:"fixVersion"`
	DictionaryID    string `json:"dictionaryId"`
	Framing         string `json:"framing"`
	Delimiter       string `json:"delimiter"`
	FrameLength     int    `json:"frameLength"`
	FrameFunctionID string `json:"frameFunctionId"`
	Sender          string `json:"sender"`
	Target          string `json:"target"`
	Heartbeat       int    `json:"heartbeat"`
	AutoLogon       bool   `json:"autoLogon"`
	ResetOnLogon    bool   `json:"resetOnLogon"`
	Username        string `json:"username"`
	Password        string `json:"password"`
}

type Request struct {
	DraftFromForm      bool           `json:"draftFromForm,omitempty"`
	PeriodSeconds      int            `json:"periodSeconds,omitempty"`
	Description        string         `json:"description,omitempty"`
	CustomNodes        *[]fix.Node    `json:"customNodes,omitempty"`
	MessageMode        string         `json:"messageMode,omitempty"`
	ID                 string         `json:"id"`
	ServiceID          string         `json:"serviceId"`
	Name               string         `json:"name"`
	Data               string         `json:"data"`
	Encoding           string         `json:"encoding"`
	PreScript          string         `json:"preScript"`
	PostScript         string         `json:"postScript"`
	RequestFunctionID  string         `json:"requestFunctionId"`
	ResponseFunctionID string         `json:"responseFunctionId"`
	MessageType        string         `json:"messageType"`
	Values             map[string]any `json:"values"`
	RawMode            bool           `json:"rawMode"`
}

type Function struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Code string `json:"code"`
}

type DictionaryFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	XML  string `json:"xml"`
}

type Workspace struct {
	Version      int              `json:"version"`
	Services     []Service        `json:"services"`
	Requests     []Request        `json:"requests"`
	Functions    []Function       `json:"functions"`
	Dictionaries []DictionaryFile `json:"dictionaries"`
}

type Event struct {
	RequestID     string          `json:"requestId,omitempty"`
	FIXDefinition *fix.Dictionary `json:"fixDefinition,omitempty"`
	ID            uint64          `json:"id"`
	Time          time.Time       `json:"time"`
	ServiceID     string          `json:"serviceId"`
	Kind          string          `json:"kind"`
	Direction     string          `json:"direction,omitempty"`
	Text          string          `json:"text"`
	Raw           string          `json:"raw,omitempty"`
	Hex           string          `json:"hex,omitempty"`
	Base64        string          `json:"base64,omitempty"`
	Size          int             `json:"size,omitempty"`
}

type HeartbeatState struct {
	Status          string     `json:"status"`
	IntervalSeconds int        `json:"intervalSeconds"`
	LastSentAt      *time.Time `json:"lastSentAt,omitempty"`
}
type TaskState struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	RequestID   string `json:"requestId,omitempty"`
	RequestName string `json:"requestName,omitempty"`
	IntervalMS  int64  `json:"intervalMs"`
}
type SessionState struct {
	Heartbeat   *HeartbeatState `json:"heartbeat,omitempty"`
	TaskDetails []TaskState     `json:"taskDetails"`
	Connected   bool            `json:"connected"`
	LoggedOn    bool            `json:"loggedOn"`
	NextSeq     int             `json:"nextSeq"`
	ExpectedSeq int             `json:"expectedSeq"`
	Tasks       []string        `json:"tasks"`
}

type Repository interface {
	Load() (Workspace, error)
	Save(Workspace) error
	Close() error
}

type Session interface {
	Send(Request) error
	StartRepeat(Request, int) error
	Close()
	State() SessionState
	StopTask(string)
}

type SessionOptions struct {
	Service    Service
	Functions  []Function
	Dictionary DictionaryFile
	Emit       func(Event)
}

type SessionFactory interface {
	Connect(SessionOptions) (Session, error)
}
