package bgg

import (
	"context"
	"fmt"
	"log"

	"github.com/DangerBlack/gobgg"
)

type BGGService interface {
	ExtractGameInfo(ctx context.Context, id int64, gameName string) (*int, *string, *string, *string, error)
	GetThings(ctx context.Context, setters ...gobgg.GetOptionSetter) ([]gobgg.ThingResult, error)
	Search(ctx context.Context, query string, setter ...gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error)
}

type bGGService struct {
	BGG *gobgg.BGG
}

func NewBGGService(bggClient *gobgg.BGG) BGGService {
	return &bGGService{
		BGG: bggClient,
	}
}

func (s *bGGService) ExtractGameInfo(ctx context.Context, id int64, gameName string) (*int, *string, *string, *string, error) {
	var err error
	var bgUrl, bgName, bgImageUrl *string
	var maxPlayers *int
	url := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", id)
	bgUrl = &url

	var things []gobgg.ThingResult

	if things, err = s.BGG.GetThings(ctx, gobgg.GetThingIDs(id)); err != nil {
		log.Printf("Failed to get game %d: %v", id, err)
		return nil, nil, nil, nil, err
	}

	if len(things) > 0 {
		maxPlayers = &things[0].MaxPlayers
		if things[0].Name != "" {
			bgName = &things[0].Name
		} else {
			bgName = &gameName
		}
		if things[0].Image != "" {
			bgImageUrl = &things[0].Image
		}
	}

	return maxPlayers, bgName, bgUrl, bgImageUrl, nil
}

func (s *bGGService) GetThings(ctx context.Context, setters ...gobgg.GetOptionSetter) ([]gobgg.ThingResult, error) {
	return s.BGG.GetThings(ctx, setters...)
}

func (s *bGGService) Search(ctx context.Context, query string, setter ...gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error) {
	return s.BGG.Search(ctx, query, setter...)
}
