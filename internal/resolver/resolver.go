package resolver

import (
	"errors"
	"strconv"
	"strings"
)

var (
	ErrUnknownName   = errors.New("unknown name")
	ErrAmbiguousName = errors.New("ambiguous name")
)

type NamedID struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier,omitempty"`
	Login      string `json:"login,omitempty"`
	Firstname  string `json:"firstname,omitempty"`
	Lastname   string `json:"lastname,omitempty"`
}

type Result struct {
	Item       NamedID
	Candidates []NamedID
	Err        error
}

func ResolveExact(items []NamedID, name string) Result {
	name = strings.TrimSpace(name)
	var matches []NamedID
	for _, item := range items {
		if exactMatch(item, name) {
			matches = append(matches, item)
		}
	}

	switch len(matches) {
	case 0:
		return Result{Err: ErrUnknownName}
	case 1:
		return Result{Item: matches[0]}
	default:
		return Result{Candidates: matches, Err: ErrAmbiguousName}
	}
}

func ResolveIDOrExact(items []NamedID, value string) Result {
	if id, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return Result{Item: NamedID{ID: id, Name: value}}
	}
	return ResolveExact(items, value)
}

func exactMatch(item NamedID, name string) bool {
	if strings.EqualFold(item.Name, name) {
		return true
	}
	if item.Identifier != "" && strings.EqualFold(item.Identifier, name) {
		return true
	}
	if item.Login != "" && strings.EqualFold(item.Login, name) {
		return true
	}
	fullName := strings.TrimSpace(strings.TrimSpace(item.Firstname) + " " + strings.TrimSpace(item.Lastname))
	return fullName != "" && strings.EqualFold(fullName, name)
}
