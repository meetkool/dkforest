package streamModals

import (
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"strings"
)

type ModalsManager struct {
	modals []IStreamModal
}

func NewModalsManager() *ModalsManager {
	return &ModalsManager{}
}

func (m *ModalsManager) Css() string {
	css := "<style>"
	for _, modal := range m.modals {
		css += modal.Css()
		css += "\n"
	}
	css += "</style>"
	return css
}

func (m *ModalsManager) Register(modal IStreamModal) {
	m.modals = append(m.modals, modal)
}

// Topics gets the unique topics of all registered modals
func (m *ModalsManager) Topics() []string {
	topics := make(map[string]bool)
	for _, modal := range m.modals {
		for _, t := range modal.Topics() {
			topics[t] = true
		}
	}
	result := make([]string, 0, len(topics))
	for k := range topics {
		result = append(result, k)
	}
	return result
}

// Handle returns after the first modal that handles a specific topic
func (m *ModalsManager) Handle(db *database.DkfDB, authUser database.IUserRenderMessage, topic, csrf string, msgTyp database.ChatMessageType, send func(string)) bool {
	for _, modal := range m.modals {
		if utils.InArr(topic, modal.Topics()) {
		
