package main

import (
	"context"
	"errors"
	"testing"

	"kwu/internal/cloud"
	"kwu/internal/expert"
)

type expertFavoritesTestClient struct {
	experts []cloud.ExpertFavorite
	groups  []cloud.ExpertGroupFavorite
}

func (c *expertFavoritesTestClient) ListExpertFavorites(context.Context, cloud.BindingScope) ([]cloud.ExpertFavorite, error) {
	return append([]cloud.ExpertFavorite(nil), c.experts...), nil
}

func (c *expertFavoritesTestClient) ListExpertGroupFavorites(context.Context, cloud.BindingScope) ([]cloud.ExpertGroupFavorite, error) {
	return append([]cloud.ExpertGroupFavorite(nil), c.groups...), nil
}

func (*expertFavoritesTestClient) CreateExpertFavorite(context.Context, cloud.BindingScope, string) (cloud.ExpertFavorite, error) {
	return cloud.ExpertFavorite{}, errors.New("unexpected create expert favorite")
}

func (*expertFavoritesTestClient) CreateExpertGroupFavorite(context.Context, cloud.BindingScope, string) (cloud.ExpertGroupFavorite, error) {
	return cloud.ExpertGroupFavorite{}, errors.New("unexpected create expert group favorite")
}

func (*expertFavoritesTestClient) DeleteExpertFavorite(context.Context, cloud.BindingScope, string) error {
	return errors.New("unexpected delete expert favorite")
}

func (*expertFavoritesTestClient) DeleteExpertGroupFavorite(context.Context, cloud.BindingScope, string) error {
	return errors.New("unexpected delete expert group favorite")
}

func TestLoadCloudExpertLibraryStateCombinesExpertAndGroupFavorites(t *testing.T) {
	client := &expertFavoritesTestClient{
		experts: []cloud.ExpertFavorite{{FavoriteID: "exfav_01", ExpertID: "exp_system"}},
		groups:  []cloud.ExpertGroupFavorite{{FavoriteID: "exgfav_01", ExpertGroupID: "exg_owned"}},
	}
	localState := expert.ExpertLibraryState{
		RecentIDs:   []string{"exp_recent"},
		UsageCounts: map[string]int64{"exp_recent": 7},
	}
	state, err := loadCloudExpertLibraryState(context.Background(), client, cloud.BindingScope{
		ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1,
	}, localState)
	if err != nil {
		t.Fatalf("loadCloudExpertLibraryState: %v", err)
	}
	if len(state.FavoriteIDs) != 2 || state.FavoriteIDs[0] != "exp_system" || state.FavoriteIDs[1] != "exg_owned" {
		t.Fatalf("favorite IDs = %v", state.FavoriteIDs)
	}
	if len(state.RecentIDs) != 1 || state.RecentIDs[0] != "exp_recent" {
		t.Fatalf("recent IDs = %v", state.RecentIDs)
	}
	localState.RecentIDs[0] = "changed"
	localState.UsageCounts["exp_recent"] = 99
	if state.RecentIDs[0] != "exp_recent" {
		t.Fatalf("state retained caller-owned recent slice: %v", state.RecentIDs)
	}
	if state.UsageCounts["exp_recent"] != 7 {
		t.Fatalf("state retained caller-owned usage counts: %v", state.UsageCounts)
	}
}
