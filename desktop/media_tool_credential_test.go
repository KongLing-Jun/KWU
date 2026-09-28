package main

import (
	"context"
	"testing"

	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestDesktopMediaToolCredentialTracksCurrentAuthorizedBinding(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	actor := userctx.ActorContext{ActorID: "media-actor", ProfileID: "media-profile"}.Normalize()
	binding := cloud.BindingScope{
		ProfileID: actor.ProfileID, CloudUserID: "media-user", BindingGeneration: 3,
	}
	done := make(chan struct{})
	manager := &desktopBillingWorkerManager{
		bindingActor:    actor,
		bindingScope:    binding,
		hasBinding:      true,
		bindingStatus:   cloud.BindingBound,
		authState:       cloud.AccountAuthenticated,
		authorized:      true,
		authorizedActor: actor,
		authorizedScope: binding,
		active: &desktopBillingWorkerProcess{
			scope: modelbilling.TaskScope{
				ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding,
			},
			done: done,
		},
	}
	catalog := &desktopSubscriptionModelCatalog{}
	catalog.replace(binding, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "maas-media-key",
	}, []string{"gpt-5.4"})
	videoClient := desktopFakeVideoGenerationClient{}
	understandingClient := desktopFakeMediaUnderstandingClient{}
	app := &App{
		billingWorkerRuntime: manager,
		subscriptionModels:   catalog,
		cloudRuntime: &cloudRuntime{
			videoGeneration: videoClient, mediaUnderstanding: understandingClient,
		},
	}

	var options boot.Options
	app.applyDesktopMediaToolCredential(&options, actor)
	if options.MediaToolCredential == nil {
		t.Fatal("MediaToolCredential was not installed")
	}
	key, err := options.MediaToolCredential(context.Background())
	if err != nil || key != "maas-media-key" {
		t.Fatalf("credential = %q, %v", key, err)
	}
	if options.MediaVideoClient == nil || options.MediaUnderstandingClient == nil || options.MediaVideoScope == nil {
		t.Fatal("Reasonix media clients and scope were not installed")
	}
	videoScope, err := options.MediaVideoScope(context.Background())
	if err != nil || videoScope != binding {
		t.Fatalf("video scope = %+v, %v", videoScope, err)
	}

	close(done)
	if key, err = options.MediaToolCredential(context.Background()); err == nil || key != "" {
		t.Fatalf("credential after worker stop = %q, %v; want unavailable", key, err)
	}
	if videoScope, err = options.MediaVideoScope(context.Background()); err == nil || videoScope != (cloud.BindingScope{}) {
		t.Fatalf("video scope after worker stop = %+v, %v; want unavailable", videoScope, err)
	}
}

type desktopFakeVideoGenerationClient struct{}

func (desktopFakeVideoGenerationClient) Create(
	context.Context,
	cloud.BindingScope,
	string,
	cloud.VideoGenerationRequest,
) (cloud.VideoGenerationTask, error) {
	return cloud.VideoGenerationTask{}, nil
}

func (desktopFakeVideoGenerationClient) Get(
	context.Context,
	cloud.BindingScope,
	string,
) (cloud.VideoGenerationTask, error) {
	return cloud.VideoGenerationTask{}, nil
}

func (desktopFakeVideoGenerationClient) GetProgress(
	context.Context,
	cloud.BindingScope,
	string,
) (cloud.VideoGenerationProgress, error) {
	return cloud.VideoGenerationProgress{}, nil
}

type desktopFakeMediaUnderstandingClient struct{}

func (desktopFakeMediaUnderstandingClient) UnderstandURL(
	context.Context,
	cloud.BindingScope,
	string,
	cloud.MediaUnderstandingRequest,
) (cloud.MediaUnderstandingResult, error) {
	return cloud.MediaUnderstandingResult{}, nil
}

func (desktopFakeMediaUnderstandingClient) UnderstandFile(
	context.Context,
	cloud.BindingScope,
	string,
	string,
	string,
	string,
	[]byte,
) (cloud.MediaUnderstandingResult, error) {
	return cloud.MediaUnderstandingResult{}, nil
}
