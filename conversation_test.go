package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConversationFlow(t *testing.T) {
	store := newFileStore(filepath.Join(t.TempDir(), "state.json"))
	manager, err := newConversationManager(store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chatID := int64(42)

	startResp := manager.handleCommand(chatID, "start")
	if !startResp.WithKeyboard {
		t.Fatalf("expected keyboard on start")
	}

	choiceResp := manager.handleMessage(chatID, "Age")
	if choiceResp.WithKeyboard {
		t.Fatalf("keyboard should not be shown while typing reply")
	}

	replyResp := manager.handleMessage(chatID, "21")
	if !replyResp.WithKeyboard {
		t.Fatalf("expected keyboard after saving reply")
	}

	state := manager.ensureState(chatID)
	if got := state.Data["Age"]; got != "21" {
		t.Fatalf("expected stored age '21', got %q", got)
	}

	doneResp := manager.handleMessage(chatID, "Done")
	if !doneResp.RemoveKeyboard {
		t.Fatalf("keyboard should be removed after done")
	}
	if !strings.Contains(doneResp.Text, "Age - 21") {
		t.Fatalf("summary missing stored fact, got: %q", doneResp.Text)
	}
}

func TestCustomCategoryPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "state.json")
	manager, err := newConversationManager(newFileStore(storePath))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chatID := int64(7)

	manager.handleCommand(chatID, "start")
	manager.handleMessage(chatID, "Something else...")
	manager.handleMessage(chatID, "Most impressive skill")
	manager.handleMessage(chatID, "Staying calm")

	reloaded, err := newConversationManager(newFileStore(storePath))
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	state := reloaded.ensureState(chatID)
	if got := state.Data["Most impressive skill"]; got != "Staying calm" {
		t.Fatalf("expected persisted custom category, got %q", got)
	}

	showResp := reloaded.handleCommand(chatID, "show_data")
	if !strings.Contains(showResp.Text, "Most impressive skill - Staying calm") {
		t.Fatalf("show_data missing content, got %q", showResp.Text)
	}
}

func TestShowAllDataCommand(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "state.json")
	manager, err := newConversationManager(newFileStore(storePath))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	firstChat := int64(1)
	secondChat := int64(2)

	manager.handleCommand(firstChat, "start")
	manager.handleMessage(firstChat, "Age")
	manager.handleMessage(firstChat, "33")

	manager.handleCommand(secondChat, "start")
	manager.handleMessage(secondChat, "Favourite colour")
	manager.handleMessage(secondChat, "Blue")

	resp := manager.handleCommand(firstChat, "show_all_data")
	if !strings.Contains(resp.Text, "User 1") || !strings.Contains(resp.Text, "Age - 33") {
		t.Fatalf("missing data for first user, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "User 2") || !strings.Contains(resp.Text, "Favourite colour - Blue") {
		t.Fatalf("missing data for second user, got %q", resp.Text)
	}
}
