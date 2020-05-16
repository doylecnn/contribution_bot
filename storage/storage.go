package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/contribution_bot/stackdriverhook"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Storage storage
type Storage struct {
	logwriter *stackdriverhook.StackdriverLoggingWriter
	logger    zerolog.Logger
	projectID string
}

// NewStorage return new storage object
func NewStorage(projectID string) Storage {
	var logger zerolog.Logger
	sw, err := stackdriverhook.NewStackdriverLoggingWriter(projectID, "storage", map[string]string{"from": "storage"})
	if err != nil {
		logger = log.Logger
		logger.Error().Err(err).Msg("new NewStackdriverLoggingWriter failed")
	} else {
		logger = zerolog.New(sw).Level(zerolog.DebugLevel)
	}

	return Storage{
		logwriter: sw,
		logger:    logger,
		projectID: projectID,
	}
}

// Close close storage object
func (s Storage) Close() {
	s.logwriter.Close()
}

// Message an article record
type Message struct {
	ID        string    `firestore:"-"`
	Username  string    `firestore:"name"`
	UserID    int       `firestore:"uid"`
	ChatID    int64     `firestore:"chatid"`
	MessageID int       `firestore:"msgid"`
	Time      time.Time `firestore:"time"`
	TimeStamp int64     `firestore:"timestamp"`
	Status    string    `firestore:"status"`
	ForwardID int       `firestore:"forwardid"`
}

// CreateNewMessage save user new message
func (s Storage) CreateNewMessage(ctx context.Context, message Message) (docRef *firestore.DocumentRef, err error) {
	client, err := firestore.NewClient(ctx, s.projectID)
	if err != nil {
		return
	}
	defer client.Close()

	message.TimeStamp = message.Time.Unix()
	docRef = client.Collection("messages").NewDoc()
	_, err = docRef.Set(ctx, message)
	return
}

// GetMessage by forwardID
func (s Storage) GetMessage(ctx context.Context, forwardID int) (originMsg Message, err error) {
	client, err := firestore.NewClient(ctx, s.projectID)
	if err != nil {
		return
	}
	defer client.Close()

	docItor := client.Collection("messages").Where("forwardid", "==", forwardID).Limit(1).Documents(ctx)
	for {
		var doc *firestore.DocumentSnapshot
		doc, err = docItor.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return
		}
		if err = doc.DataTo(&originMsg); err != nil {
			s.logger.Error().Err(err).Send()
			return
		}
		originMsg.ID = doc.Ref.ID
		return
	}
	return
}

// UpdateMessageStatus update message status
func (s Storage) UpdateMessageStatus(ctx context.Context, docRef *firestore.DocumentRef, message Message) (err error) {
	client, err := firestore.NewClient(ctx, s.projectID)
	if err != nil {
		return
	}
	defer client.Close()

	batch := client.Batch()
	batch.Update(docRef, []firestore.Update{
		{Path: "forwardid", Value: message.ForwardID},
		{Path: "status", Value: message.Status},
	})
	_, err = batch.Commit(ctx)
	return
}

//DeleteOldForwardMessages delete old messages
func (s Storage) DeleteOldForwardMessages(ctx context.Context) (err error) {
	client, err := firestore.NewClient(ctx, s.projectID)
	if err != nil {
		return
	}
	defer client.Close()

	var docRefs []*firestore.DocumentRef
	docItor := client.Collection("messages").Where("timestamp", "<", time.Now().Add(-3*24*time.Hour).Unix()).Where("status", "==", "forward").Documents(ctx)
	for {
		var doc *firestore.DocumentSnapshot
		doc, err = docItor.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.logger.Error().Err(err).Send()
			return
		}
		if doc.Exists() {
			docRefs = append(docRefs, doc.Ref)
		}
	}

	batch := client.Batch()
	deleteItem := 0
	for _, msg := range docRefs {
		batch.Delete(msg)
		deleteItem++
	}
	if deleteItem > 0 {
		_, err = batch.Commit(ctx)
		if err != nil {
			s.logger.Error().Err(err).Send()
		}
	}
	return
}

// Settings bot settings
type Settings struct {
	WelcomeWords           string `firestore:"welcome_words"`
	Thanks                 string `firestore:"thanks"`
	ForwardMessageToChatID int64  `firestore:"forward_message_to_chat_id"`
	BotInfo                string `firestore:"bot_info"`
}

func (s Settings) String() string {
	return fmt.Sprintf("bot info: %s\nwelcome words: %s\nthanks words: %s\nforward to: %d",
		s.BotInfo,
		s.WelcomeWords,
		s.Thanks,
		s.ForwardMessageToChatID,
	)
}

// SaveSettings save settings
func (s Storage) SaveSettings(ctx context.Context, settings Settings) (err error) {
	client, err := firestore.NewClient(ctx, s.projectID)
	if err != nil {
		return
	}
	defer client.Close()

	docRef := client.Doc("settings/setting")
	docSnap, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			s.logger.Error().Err(err).Send()
			return
		}
		// status.Code(err) == codes.NotFound
		_, err = docRef.Create(ctx, settings)
		return
	}
	if docSnap.Exists() {
		var oldSettings Settings
		if err = docSnap.DataTo(&oldSettings); err != nil {
			s.logger.Error().Err(err).Send()
			return
		}
		var batch = client.Batch()
		var updates []firestore.Update
		if oldSettings.BotInfo != settings.BotInfo {
			updates = append(updates, firestore.Update{Path: "bot_info", Value: settings.BotInfo})
		}
		if oldSettings.WelcomeWords != settings.WelcomeWords {
			updates = append(updates, firestore.Update{Path: "welcome_words", Value: settings.WelcomeWords})
		}
		if oldSettings.Thanks != settings.Thanks {
			updates = append(updates, firestore.Update{Path: "thanks", Value: settings.Thanks})
		}
		if oldSettings.ForwardMessageToChatID != settings.ForwardMessageToChatID {
			updates = append(updates, firestore.Update{Path: "forward_message_to_chat_id", Value: settings.ForwardMessageToChatID})
		}
		batch.Update(docRef, updates)
		_, err = batch.Commit(ctx)
		if err != nil {
			s.logger.Error().Err(err).Send()
		}
	} else {
		err = errors.New("settings not found")
		s.logger.Error().Err(err).Send()
	}
	return
}

//GetSettings get settings
func (s Storage) GetSettings(ctx context.Context) (settings Settings, err error) {
	client, err := firestore.NewClient(ctx, s.projectID)
	if err != nil {
		s.logger.Error().Err(err).Send()
		return
	}
	defer client.Close()

	docSnap, err := client.Doc("settings/setting").Get(ctx)
	if err != nil {
		s.logger.Error().Err(err).Send()
		return
	}
	if !docSnap.Exists() {
		err = errors.New("settings not found")
		s.logger.Error().Err(err).Send()
		return
	}
	err = docSnap.DataTo(&settings)
	if err != nil {
		s.logger.Error().Err(err).Send()
	}
	return
}
