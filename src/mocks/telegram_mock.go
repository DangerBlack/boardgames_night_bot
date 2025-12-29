package mocks

import (
	"boardgame-night-bot/src/telegram"
	"io"
	"strings"

	"gopkg.in/telebot.v3"
)

type MockTelegramService struct {
	SendFunc   func(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error)
	DeleteFunc func(msg telebot.Editable) error
}

func NewMockTelegramService() *MockTelegramService {
	return &MockTelegramService{}
}

func (m *MockTelegramService) Accept(query *telebot.PreCheckoutQuery, errorMessage ...string) error {
	return nil
}

func (m *MockTelegramService) AddStickerToSet(of telebot.Recipient, name string, sticker telebot.InputSticker) error {
	return nil
}

func (m *MockTelegramService) AdminsOf(chat *telebot.Chat) ([]telebot.ChatMember, error) {
	return []telebot.ChatMember{}, nil
}

func (m *MockTelegramService) Answer(query *telebot.Query, resp *telebot.QueryResponse) error {
	return nil
}

func (m *MockTelegramService) AnswerWebApp(query *telebot.Query, r telebot.Result) (*telebot.WebAppMessage, error) {
	return &telebot.WebAppMessage{}, nil
}

func (m *MockTelegramService) ApproveJoinRequest(chat telebot.Recipient, user *telebot.User) error {
	return nil
}

func (m *MockTelegramService) Ban(chat *telebot.Chat, member *telebot.ChatMember, revokeMessages ...bool) error {
	return nil
}

func (m *MockTelegramService) BanSenderChat(chat *telebot.Chat, sender telebot.Recipient) error {
	return nil
}

func (m *MockTelegramService) ChatByID(id int64) (*telebot.Chat, error) {
	return &telebot.Chat{ID: id}, nil
}

func (m *MockTelegramService) ChatByUsername(name string) (*telebot.Chat, error) {
	return &telebot.Chat{Username: name}, nil
}

func (m *MockTelegramService) ChatMemberOf(chat telebot.Recipient, user telebot.Recipient) (*telebot.ChatMember, error) {
	return &telebot.ChatMember{}, nil
}

func (m *MockTelegramService) Close() (bool, error) {
	return true, nil
}

func (m *MockTelegramService) CloseGeneralTopic(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) CloseTopic(chat *telebot.Chat, topic *telebot.Topic) error {
	return nil
}

func (m *MockTelegramService) Commands(opts ...interface{}) ([]telebot.Command, error) {
	return []telebot.Command{}, nil
}

func (m *MockTelegramService) Copy(to telebot.Recipient, msg telebot.Editable, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) CopyMany(to telebot.Recipient, msgs []telebot.Editable, opts ...*telebot.SendOptions) ([]telebot.Message, error) {
	return []telebot.Message{}, nil
}

func (m *MockTelegramService) CreateInviteLink(chat telebot.Recipient, link *telebot.ChatInviteLink) (*telebot.ChatInviteLink, error) {
	return &telebot.ChatInviteLink{}, nil
}

func (m *MockTelegramService) CreateInvoiceLink(i telebot.Invoice) (string, error) {
	return "mock-invoice-link", nil
}

func (m *MockTelegramService) CreateStickerSet(of telebot.Recipient, set *telebot.StickerSet) error {
	return nil
}

func (m *MockTelegramService) CreateTopic(chat *telebot.Chat, topic *telebot.Topic) (*telebot.Topic, error) {
	return &telebot.Topic{}, nil
}

func (m *MockTelegramService) CustomEmojiStickers(ids []string) ([]telebot.Sticker, error) {
	return []telebot.Sticker{}, nil
}

func (m *MockTelegramService) DeclineJoinRequest(chat telebot.Recipient, user *telebot.User) error {
	return nil
}

func (m *MockTelegramService) DefaultRights(forChannels bool) (*telebot.Rights, error) {
	return &telebot.Rights{}, nil
}

func (m *MockTelegramService) Delete(msg telebot.Editable) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(msg)
	}
	return nil
}

func (m *MockTelegramService) DeleteCommands(opts ...interface{}) error {
	return nil
}

func (m *MockTelegramService) DeleteGroupPhoto(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) DeleteGroupStickerSet(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) DeleteMany(msgs []telebot.Editable) error {
	return nil
}

func (m *MockTelegramService) DeleteSticker(sticker string) error {
	return nil
}

func (m *MockTelegramService) DeleteStickerSet(name string) error {
	return nil
}

func (m *MockTelegramService) DeleteTopic(chat *telebot.Chat, topic *telebot.Topic) error {
	return nil
}

func (m *MockTelegramService) Download(file *telebot.File, localFilename string) error {
	return nil
}

func (m *MockTelegramService) Edit(msg telebot.Editable, what interface{}, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) EditCaption(msg telebot.Editable, caption string, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) EditGeneralTopic(chat *telebot.Chat, topic *telebot.Topic) error {
	return nil
}

func (m *MockTelegramService) EditInviteLink(chat telebot.Recipient, link *telebot.ChatInviteLink) (*telebot.ChatInviteLink, error) {
	return &telebot.ChatInviteLink{}, nil
}

func (m *MockTelegramService) EditMedia(msg telebot.Editable, media telebot.Inputtable, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) EditReplyMarkup(msg telebot.Editable, markup *telebot.ReplyMarkup) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) EditTopic(chat *telebot.Chat, topic *telebot.Topic) error {
	return nil
}

func (m *MockTelegramService) File(file *telebot.File) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockTelegramService) FileByID(fileID string) (telebot.File, error) {
	return telebot.File{FileID: fileID}, nil
}

func (m *MockTelegramService) Forward(to telebot.Recipient, msg telebot.Editable, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) ForwardMany(to telebot.Recipient, msgs []telebot.Editable, opts ...*telebot.SendOptions) ([]telebot.Message, error) {
	return []telebot.Message{}, nil
}

func (m *MockTelegramService) GameScores(user telebot.Recipient, msg telebot.Editable) ([]telebot.GameHighScore, error) {
	return []telebot.GameHighScore{}, nil
}

func (m *MockTelegramService) Group() *telebot.Group {
	return &telebot.Group{}
}

func (m *MockTelegramService) Handle(endpoint interface{}, h telebot.HandlerFunc, mws ...telebot.MiddlewareFunc) {
}

func (m *MockTelegramService) HideGeneralTopic(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) InviteLink(chat *telebot.Chat) (string, error) {
	return "mock-invite-link", nil
}

func (m *MockTelegramService) Leave(chat telebot.Recipient) error {
	return nil
}

func (m *MockTelegramService) Len(chat *telebot.Chat) (int, error) {
	return 0, nil
}

func (m *MockTelegramService) Logout() (bool, error) {
	return true, nil
}

func (m *MockTelegramService) MenuButton(chat *telebot.User) (*telebot.MenuButton, error) {
	return &telebot.MenuButton{}, nil
}

func (m *MockTelegramService) MyDescription(language string) (*telebot.BotInfo, error) {
	return &telebot.BotInfo{}, nil
}

func (m *MockTelegramService) MyName(language string) (*telebot.BotInfo, error) {
	return &telebot.BotInfo{}, nil
}

func (m *MockTelegramService) MyShortDescription(language string) (*telebot.BotInfo, error) {
	return &telebot.BotInfo{}, nil
}

func (m *MockTelegramService) NewContext(u telebot.Update) telebot.Context {
	return nil
}

func (m *MockTelegramService) NewMarkup() *telebot.ReplyMarkup {
	return &telebot.ReplyMarkup{}
}

func (m *MockTelegramService) Notify(to telebot.Recipient, action telebot.ChatAction, threadID ...int) error {
	return nil
}

func (m *MockTelegramService) OnError(err error, c telebot.Context) {}

func (m *MockTelegramService) Pin(msg telebot.Editable, opts ...interface{}) error {
	return nil
}

func (m *MockTelegramService) ProcessUpdate(u telebot.Update) {}

func (m *MockTelegramService) ProfilePhotosOf(user *telebot.User) ([]telebot.Photo, error) {
	return []telebot.Photo{}, nil
}

func (m *MockTelegramService) Promote(chat *telebot.Chat, member *telebot.ChatMember) error {
	return nil
}

func (m *MockTelegramService) Raw(method string, payload interface{}) ([]byte, error) {
	return []byte{}, nil
}

func (m *MockTelegramService) React(to telebot.Recipient, msg telebot.Editable, opts ...telebot.ReactionOptions) error {
	return nil
}

func (m *MockTelegramService) RemoveWebhook(dropPending ...bool) error {
	return nil
}

func (m *MockTelegramService) ReopenGeneralTopic(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) ReopenTopic(chat *telebot.Chat, topic *telebot.Topic) error {
	return nil
}

func (m *MockTelegramService) Reply(to *telebot.Message, what interface{}, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) Respond(c *telebot.Callback, resp ...*telebot.CallbackResponse) error {
	return nil
}

func (m *MockTelegramService) Restrict(chat *telebot.Chat, member *telebot.ChatMember) error {
	return nil
}

func (m *MockTelegramService) RevokeInviteLink(chat telebot.Recipient, link string) (*telebot.ChatInviteLink, error) {
	return &telebot.ChatInviteLink{}, nil
}

func (m *MockTelegramService) Send(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error) {
	if m.SendFunc != nil {
		return m.SendFunc(to, what, opts...)
	}
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) SendAlbum(to telebot.Recipient, a telebot.Album, opts ...interface{}) ([]telebot.Message, error) {
	return []telebot.Message{}, nil
}

func (m *MockTelegramService) SetAdminTitle(chat *telebot.Chat, user *telebot.User, title string) error {
	return nil
}
func (m *MockTelegramService) SetCommands(opts ...interface{}) error {
	return nil
}
func (m *MockTelegramService) SetCustomEmojiStickerSetThumb(name string, id string) error {
	return nil
}
func (m *MockTelegramService) SetDefaultRights(rights telebot.Rights, forChannels bool) error {
	return nil
}
func (m *MockTelegramService) SetGameScore(user telebot.Recipient, msg telebot.Editable, score telebot.GameHighScore) (*telebot.Message, error) {
	return nil, nil
}
func (m *MockTelegramService) SetGroupDescription(chat *telebot.Chat, description string) error {
	return nil
}
func (m *MockTelegramService) SetGroupPermissions(chat *telebot.Chat, perms telebot.Rights) error {
	return nil
}
func (m *MockTelegramService) SetGroupPhoto(chat *telebot.Chat, p *telebot.Photo) error {
	return nil
}
func (m *MockTelegramService) SetGroupStickerSet(chat *telebot.Chat, setName string) error {
	return nil
}
func (m *MockTelegramService) SetGroupTitle(chat *telebot.Chat, title string) error {
	return nil
}
func (m *MockTelegramService) SetMenuButton(chat *telebot.User, mb interface{}) error {
	return nil
}
func (m *MockTelegramService) SetMyDescription(desc string, language string) error {
	return nil
}
func (m *MockTelegramService) SetMyName(name string, language string) error {
	return nil
}
func (m *MockTelegramService) SetMyShortDescription(desc string, language string) error {
	return nil
}
func (m *MockTelegramService) SetStickerEmojis(sticker string, emojis []string) error {
	return nil
}
func (m *MockTelegramService) SetStickerKeywords(sticker string, keywords []string) error {
	return nil
}
func (m *MockTelegramService) SetStickerMaskPosition(sticker string, mask telebot.MaskPosition) error {
	return nil
}
func (m *MockTelegramService) SetStickerPosition(sticker string, position int) error {
	return nil
}
func (m *MockTelegramService) SetStickerSetThumb(of telebot.Recipient, set *telebot.StickerSet) error {
	return nil
}
func (m *MockTelegramService) SetStickerSetTitle(s telebot.StickerSet) error {
	return nil
}
func (m *MockTelegramService) SetWebhook(w *telebot.Webhook) error {
	return nil
}
func (m *MockTelegramService) Ship(query *telebot.ShippingQuery, what ...interface{}) error {
	return nil
}

func (m *MockTelegramService) Start() {}

func (m *MockTelegramService) Stop() {}

func (m *MockTelegramService) StickerSet(name string) (*telebot.StickerSet, error) {
	return &telebot.StickerSet{Name: name}, nil
}

func (m *MockTelegramService) StopLiveLocation(msg telebot.Editable, opts ...interface{}) (*telebot.Message, error) {
	return &telebot.Message{}, nil
}

func (m *MockTelegramService) StopPoll(msg telebot.Editable, opts ...interface{}) (*telebot.Poll, error) {
	return &telebot.Poll{}, nil
}

func (m *MockTelegramService) TopicIconStickers() ([]telebot.Sticker, error) {
	return []telebot.Sticker{}, nil
}

func (m *MockTelegramService) Trigger(endpoint interface{}, c telebot.Context) error {
	return nil
}

func (m *MockTelegramService) Unban(chat *telebot.Chat, user *telebot.User, forBanned ...bool) error {
	return nil
}

func (m *MockTelegramService) UnbanSenderChat(chat *telebot.Chat, sender telebot.Recipient) error {
	return nil
}

func (m *MockTelegramService) UnhideGeneralTopic(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) Unpin(chat telebot.Recipient, messageID ...int) error {
	return nil
}

func (m *MockTelegramService) UnpinAll(chat telebot.Recipient) error {
	return nil
}

func (m *MockTelegramService) UnpinAllGeneralTopicMessages(chat *telebot.Chat) error {
	return nil
}

func (m *MockTelegramService) UnpinAllTopicMessages(chat *telebot.Chat, topic *telebot.Topic) error {
	return nil
}

func (m *MockTelegramService) UploadSticker(to telebot.Recipient, format telebot.StickerSetFormat, f telebot.File) (*telebot.File, error) {
	return &telebot.File{}, nil
}

func (m *MockTelegramService) Use(middleware ...telebot.MiddlewareFunc) {}

func (m *MockTelegramService) UserBoosts(chat telebot.Recipient, user telebot.Recipient) ([]telebot.Boost, error) {
	return []telebot.Boost{}, nil
}

func (m *MockTelegramService) Webhook() (*telebot.Webhook, error) {
	return &telebot.Webhook{}, nil
}

var _ telegram.TelegramService = &MockTelegramService{}
