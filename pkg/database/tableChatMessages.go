package database

import (
	"crypto/cipher"
	"crypto/rand"
	"dkforest/pkg/config"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"io"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ChatMessages []ChatMessage

func (m *ChatMessage) Decrypt(key string) error {
	aesgcm, _, err := utils.GetGCM(key)
	if err != nil {
		return err
	}
	m.Message = decrypt(m.Message, aesgcm)
	return nil
}

func (m ChatMessages) DecryptAll(key string) error {
	aesgcm, _, err := utils.GetGCM(key)
	if err != nil {
		return err
	}
	for i := range m {
		m[i].Message = decrypt(m[i].Message, aesgcm)
	}
	return nil
}

func (m ChatMessages) DecryptAllRaw(key string) error {
	aesgcm, _, err := utils.GetGCM(key)
	if err != nil {
	
