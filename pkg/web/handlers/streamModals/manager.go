package streamModals

import (
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
)

type ModalsManager struct {
	modals []IStreamModal
}

func NewModalsManager() *ModalsManager {
	return &ModalsManager{}
}

func (m *ModalsManager) Css() (out string) {
	out = "<style>"
	for _, modal := range m.modals {
		out += modal.Css()
		out += "\n"
	}
	out += "</style>"
	return
}

func (m *ModalsManager) Register(modal IStreamModal) {
	m.modals = append(m.modals, modal)
}

// Topics gets the topics of all registered modals
func (m *ModalsManager) Topics() (out []string) {
	for _, modal := range m.modals {
		out = append(out, modal.Topics()...)
	}
	return
}

// Handle returns after the first modal that handle a specific topic
func (m *ModalsManager) Handle(db *database.DkfDB, authUser database.IUserRenderMessage, topic, csrf string, msgTyp database.ChatMessageType, send func(string)) bool {
	for _, modal := range m.modals {
		if utils.InArr(topic, modal.Topics()) {
			if modal.Handle(db, authUser, topic, csrf, msgTyp, send) {
				return true
			}
		}
	}
	return false
}
