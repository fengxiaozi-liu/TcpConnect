package domain

import "tcpconnect/internal/domain/fix"

func (r Request) Definition(d *fix.Dictionary) (*fix.Dictionary, error) {
	return d.WithMessageNodes(r.MessageType, r.CustomNodes)
}
func (r Request) Body(d *fix.Dictionary) ([]fix.Pair, error) {
	d, err := r.Definition(d)
	if err != nil {
		return nil, err
	}
	if r.MessageMode == "text" {
		return d.TextBody(r.MessageType, r.Data)
	}
	return d.Body(r.MessageType, r.Values)
}
