package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	stageChoosing    = "CHOOSING"
	stageTypingReply = "TYPING_REPLY"
	stageTypingKey   = "TYPING_CHOICE"
)

var mainKeyboardOptions = []string{
	"Age",
	"Favourite colour",
	"Number of siblings",
	"Something else...",
	"Done",
}

type userState struct {
	Stage      string            `json:"stage"`
	Data       map[string]string `json:"data"`
	PendingKey string            `json:"pending_key"`
}

type response struct {
	Text           string
	WithKeyboard   bool
	RemoveKeyboard bool
}

type conversationManager struct {
	states map[int64]*userState
	store  *fileStore
	mu     sync.Mutex
}

func newConversationManager(store *fileStore) (*conversationManager, error) {
	states, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &conversationManager{
		states: states,
		store:  store,
	}, nil
}

func (m *conversationManager) handleCommand(chatID int64, command string) response {
	switch command {
	case "start":
		return m.startConversation(chatID)
	case "cancel", "stop":
		return m.finishConversation(chatID)
	case "show_data":
		state := m.ensureState(chatID)
		return response{Text: "Stored facts:" + factsToStr(state.Data), WithKeyboard: true}
	case "show_all_data":
		return response{Text: m.allUsersDataSummary()}
	default:
		return response{Text: "Unknown command. Try /start to begin.", WithKeyboard: false}
	}
}

func (m *conversationManager) handleMessage(chatID int64, text string) response {
	state := m.ensureState(chatID)
	switch state.Stage {
	case stageTypingKey:
		return m.handleTypingKey(chatID, text)
	case stageTypingReply:
		return m.handleTypingReply(chatID, text)
	default:
		return m.handleChoosing(chatID, text)
	}
}

func (m *conversationManager) handleChoosing(chatID int64, text string) response {
	state := m.ensureState(chatID)
	clean := strings.TrimSpace(text)
	switch clean {
	case "Something else...":
		state.Stage = stageTypingKey
		m.saveStateLocked(chatID, state)
		return response{Text: "Tell me the category name.", WithKeyboard: false}
	case "Done":
		return m.finishConversation(chatID)
	}

	for _, option := range mainKeyboardOptions {
		if clean == option && option != "Something else..." && option != "Done" {
			state.Stage = stageTypingReply
			state.PendingKey = option
			m.saveStateLocked(chatID, state)
			return response{Text: fmt.Sprintf("Your %s? Please type it.", option), WithKeyboard: false}
		}
	}

	return response{
		Text:         "Please choose one of the options on the keyboard.",
		WithKeyboard: true,
	}
}

func (m *conversationManager) handleTypingKey(chatID int64, text string) response {
	state := m.ensureState(chatID)
	key := strings.TrimSpace(text)
	if key == "" {
		return response{Text: "Category cannot be empty. Please send a label for your data."}
	}
	state.Stage = stageTypingReply
	state.PendingKey = key
	m.saveStateLocked(chatID, state)
	return response{Text: fmt.Sprintf("Great, now tell me about %s.", key)}
}

func (m *conversationManager) handleTypingReply(chatID int64, text string) response {
	state := m.ensureState(chatID)
	key := state.PendingKey
	if key == "" {
		state.Stage = stageChoosing
		m.saveStateLocked(chatID, state)
		return response{Text: "I lost track of what we were talking about. Pick an option again.", WithKeyboard: true}
	}

	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	state.Data[key] = strings.TrimSpace(text)
	state.PendingKey = ""
	state.Stage = stageChoosing
	m.saveStateLocked(chatID, state)
	return response{
		Text:         fmt.Sprintf("Saved %s. What would you like to do next?", key),
		WithKeyboard: true,
	}
}

func (m *conversationManager) startConversation(chatID int64) response {
	state := m.ensureState(chatID)
	state.Stage = stageChoosing
	m.saveStateLocked(chatID, state)
	greeting := "Hi! My name is Doctor Botter."
	if len(state.Data) > 0 {
		greeting += fmt.Sprintf(
			" You already told me your %s. Tell me more or update something.",
			strings.Join(sortedKeys(state.Data), ", "),
		)
	} else {
		greeting += " I will hold a more complex conversation with you. Tell me something about yourself."
	}
	return response{
		Text:         greeting,
		WithKeyboard: true,
	}
}

func (m *conversationManager) finishConversation(chatID int64) response {
	state := m.ensureState(chatID)
	summary := "I learned these facts about you:" + factsToStr(state.Data) + "Until next time!"
	state.Stage = stageChoosing
	state.PendingKey = ""
	m.saveStateLocked(chatID, state)
	return response{
		Text:           summary,
		RemoveKeyboard: true,
	}
}

func (m *conversationManager) ensureState(chatID int64) *userState {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[chatID]
	if !ok {
		state = &userState{
			Stage: stageChoosing,
			Data:  make(map[string]string),
		}
		m.states[chatID] = state
	}
	return state
}

func (m *conversationManager) saveStateLocked(chatID int64, state *userState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[chatID] = state
	if err := m.store.Save(m.states); err != nil {
		log.Printf("failed to persist state for %d: %v", chatID, err)
	}
}

func (m *conversationManager) allUsersDataSummary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.states) == 0 {
		return "All saved data:\n(nothing yet)\n"
	}

	ids := make([]int64, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var parts []string
	for _, id := range ids {
		state := m.states[id]
		parts = append(parts, fmt.Sprintf("User %d:%s", id, factsToStr(state.Data)))
	}
	return "All saved data:\n" + strings.Join(parts, "\n")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func factsToStr(userData map[string]string) string {
	if len(userData) == 0 {
		return "\n(nothing yet)\n"
	}
	keys := sortedKeys(userData)
	var facts []string
	for _, k := range keys {
		facts = append(facts, fmt.Sprintf("%s - %s", k, userData[k]))
	}
	return "\n" + strings.Join(facts, "\n") + "\n"
}

type fileStore struct {
	path string
}

func newFileStore(path string) *fileStore {
	return &fileStore{path: path}
}

func (f *fileStore) Load() (map[int64]*userState, error) {
	states := make(map[int64]*userState)
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return states, nil
	}
	if err != nil {
		return nil, err
	}

	var encoded map[string]*userState
	if err := json.Unmarshal(data, &encoded); err != nil {
		return nil, err
	}

	for k, v := range encoded {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		states[id] = v
	}
	return states, nil
}

func (f *fileStore) Save(states map[int64]*userState) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	encoded := make(map[string]*userState, len(states))
	for k, v := range states {
		encoded[strconv.FormatInt(k, 10)] = v
	}
	data, err := json.MarshalIndent(encoded, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}
