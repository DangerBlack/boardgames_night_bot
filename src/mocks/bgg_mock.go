package mocks

import (
	"context"
	"log"

	"github.com/DangerBlack/gobgg"
)

type MockBGGService struct {
	ExtractGameInfoFunc func(ctx context.Context, id int64, gameName string) (*int, *string, *string, *string, error)
	GetThingsFunc       func(ctx context.Context, setters []gobgg.GetOptionSetter) ([]gobgg.ThingResult, error)
	SearchFunc          func(ctx context.Context, query string, setter []gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error)
}

func NewMockBGGService() *MockBGGService {
	return &MockBGGService{}
}

func (m *MockBGGService) ExtractGameInfo(ctx context.Context, id int64, gameName string) (*int, *string, *string, *string, error) {
	if m.ExtractGameInfoFunc != nil {
		return m.ExtractGameInfoFunc(ctx, id, gameName)
	}
	log.Println("MockBGGService.ExtractGameInfo callback not configured")
	return nil, nil, nil, nil, nil
}

func (m *MockBGGService) GetThings(ctx context.Context, setters ...gobgg.GetOptionSetter) ([]gobgg.ThingResult, error) {
	if m.GetThingsFunc != nil {
		return m.GetThingsFunc(ctx, setters)
	}
	log.Println("MockBGGService.GetThings callback not configured")
	return []gobgg.ThingResult{}, nil
}

func (m *MockBGGService) Search(ctx context.Context, query string, setter ...gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query, setter)
	}
	log.Println("MockBGGService.Search callback not configured")
	return []gobgg.SearchResult{}, nil
}
