package bgg

import (
	"boardgame-night-bot/src/models"
	"context"
	"fmt"
	"log"

	"github.com/DangerBlack/gobgg"
	"github.com/bluele/gcache"
)

type BGGService interface {
	ExtractGameInfo(ctx context.Context, id int64, gameName string) (*models.BggInfo, error)
	ExtractCachedGameInfo(ctx context.Context, id int64, gameName string) (*models.BggInfo, error)
	GetThings(ctx context.Context, setters ...gobgg.GetOptionSetter) ([]gobgg.ThingResult, error)
	Search(ctx context.Context, query string, setter ...gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error)
}

type bGGService struct {
	BGG   *gobgg.BGG
	cache gcache.Cache
}

func NewBGGService(bggClient *gobgg.BGG) BGGService {
	cache := gcache.New(1000).LRU().Build()
	return &bGGService{
		BGG:   bggClient,
		cache: cache,
	}
}

func (s *bGGService) ExtractCachedGameInfo(ctx context.Context, id int64, gameName string) (*models.BggInfo, error) {
	cacheKey := fmt.Sprintf("bgg_info_%d", id)
	if cached, err := s.cache.Get(cacheKey); err == nil {
		if info, ok := cached.(*models.BggInfo); ok {
			return info, nil
		}
	}

	info, err := s.ExtractGameInfo(ctx, id, gameName)
	if err != nil {
		return nil, err
	}

	if err := s.cache.Set(cacheKey, info); err != nil {
		log.Default().Printf("Failed to cache BGG info for game %d: %v", id, err)
	}

	return info, nil
}

func (s *bGGService) ExtractGameInfo(ctx context.Context, id int64, gameName string) (*models.BggInfo, error) {
	var err error
	var bgUrl, bgName, bgImageUrl *string
	var maxPlayers *int
	url := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", id)
	bgUrl = &url

	var things []gobgg.ThingResult

	if things, err = s.BGG.GetThings(ctx, gobgg.GetThingIDs(id)); err != nil {
		log.Default().Printf("Failed to get game %d: %v", id, err)
		return nil, err
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

	info := models.BggInfo{
		Name:       bgName,
		Url:        bgUrl,
		ImageUrl:   bgImageUrl,
		MaxPlayers: maxPlayers,
	}

	return &info, nil
}

func (s *bGGService) GetThings(ctx context.Context, setters ...gobgg.GetOptionSetter) ([]gobgg.ThingResult, error) {
	return s.BGG.GetThings(ctx, setters...)
}

func (s *bGGService) Search(ctx context.Context, query string, setter ...gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error) {
	return s.BGG.Search(ctx, query, setter...)
}
