package telegram

import (
	"io"

	"gopkg.in/telebot.v3"
)

type TelegramService interface {
	Accept(query *telebot.PreCheckoutQuery, errorMessage ...string) error
	AddStickerToSet(of telebot.Recipient, name string, sticker telebot.InputSticker) error
	AdminsOf(chat *telebot.Chat) ([]telebot.ChatMember, error)
	Answer(query *telebot.Query, resp *telebot.QueryResponse) error
	AnswerWebApp(query *telebot.Query, r telebot.Result) (*telebot.WebAppMessage, error)
	ApproveJoinRequest(chat telebot.Recipient, user *telebot.User) error
	Ban(chat *telebot.Chat, member *telebot.ChatMember, revokeMessages ...bool) error
	BanSenderChat(chat *telebot.Chat, sender telebot.Recipient) error
	ChatByID(id int64) (*telebot.Chat, error)
	ChatByUsername(name string) (*telebot.Chat, error)
	ChatMemberOf(chat telebot.Recipient, user telebot.Recipient) (*telebot.ChatMember, error)
	Close() (bool, error)
	CloseGeneralTopic(chat *telebot.Chat) error
	CloseTopic(chat *telebot.Chat, topic *telebot.Topic) error
	Commands(opts ...interface{}) ([]telebot.Command, error)
	Copy(to telebot.Recipient, msg telebot.Editable, opts ...interface{}) (*telebot.Message, error)
	CopyMany(to telebot.Recipient, msgs []telebot.Editable, opts ...*telebot.SendOptions) ([]telebot.Message, error)
	CreateInviteLink(chat telebot.Recipient, link *telebot.ChatInviteLink) (*telebot.ChatInviteLink, error)
	CreateInvoiceLink(i telebot.Invoice) (string, error)
	CreateStickerSet(of telebot.Recipient, set *telebot.StickerSet) error
	CreateTopic(chat *telebot.Chat, topic *telebot.Topic) (*telebot.Topic, error)
	CustomEmojiStickers(ids []string) ([]telebot.Sticker, error)
	DeclineJoinRequest(chat telebot.Recipient, user *telebot.User) error
	DefaultRights(forChannels bool) (*telebot.Rights, error)
	Delete(msg telebot.Editable) error
	DeleteCommands(opts ...interface{}) error
	DeleteGroupPhoto(chat *telebot.Chat) error
	DeleteGroupStickerSet(chat *telebot.Chat) error
	DeleteMany(msgs []telebot.Editable) error
	DeleteSticker(sticker string) error
	DeleteStickerSet(name string) error
	DeleteTopic(chat *telebot.Chat, topic *telebot.Topic) error
	Download(file *telebot.File, localFilename string) error
	Edit(msg telebot.Editable, what interface{}, opts ...interface{}) (*telebot.Message, error)
	EditCaption(msg telebot.Editable, caption string, opts ...interface{}) (*telebot.Message, error)
	EditGeneralTopic(chat *telebot.Chat, topic *telebot.Topic) error
	EditInviteLink(chat telebot.Recipient, link *telebot.ChatInviteLink) (*telebot.ChatInviteLink, error)
	EditMedia(msg telebot.Editable, media telebot.Inputtable, opts ...interface{}) (*telebot.Message, error)
	EditReplyMarkup(msg telebot.Editable, markup *telebot.ReplyMarkup) (*telebot.Message, error)
	EditTopic(chat *telebot.Chat, topic *telebot.Topic) error
	File(file *telebot.File) (io.ReadCloser, error)
	FileByID(fileID string) (telebot.File, error)
	Forward(to telebot.Recipient, msg telebot.Editable, opts ...interface{}) (*telebot.Message, error)
	ForwardMany(to telebot.Recipient, msgs []telebot.Editable, opts ...*telebot.SendOptions) ([]telebot.Message, error)
	GameScores(user telebot.Recipient, msg telebot.Editable) ([]telebot.GameHighScore, error)
	Group() *telebot.Group
	Handle(endpoint interface{}, h telebot.HandlerFunc, m ...telebot.MiddlewareFunc)
	HideGeneralTopic(chat *telebot.Chat) error
	InviteLink(chat *telebot.Chat) (string, error)
	Leave(chat telebot.Recipient) error
	Len(chat *telebot.Chat) (int, error)
	Logout() (bool, error)
	MenuButton(chat *telebot.User) (*telebot.MenuButton, error)
	MyDescription(language string) (*telebot.BotInfo, error)
	MyName(language string) (*telebot.BotInfo, error)
	MyShortDescription(language string) (*telebot.BotInfo, error)
	NewContext(u telebot.Update) telebot.Context
	NewMarkup() *telebot.ReplyMarkup
	Notify(to telebot.Recipient, action telebot.ChatAction, threadID ...int) error
	OnError(err error, c telebot.Context)
	Pin(msg telebot.Editable, opts ...interface{}) error
	ProcessUpdate(u telebot.Update)
	ProfilePhotosOf(user *telebot.User) ([]telebot.Photo, error)
	Promote(chat *telebot.Chat, member *telebot.ChatMember) error
	Raw(method string, payload interface{}) ([]byte, error)
	React(to telebot.Recipient, msg telebot.Editable, opts ...telebot.ReactionOptions) error
	RemoveWebhook(dropPending ...bool) error
	ReopenGeneralTopic(chat *telebot.Chat) error
	ReopenTopic(chat *telebot.Chat, topic *telebot.Topic) error
	Reply(to *telebot.Message, what interface{}, opts ...interface{}) (*telebot.Message, error)
	Respond(c *telebot.Callback, resp ...*telebot.CallbackResponse) error
	Restrict(chat *telebot.Chat, member *telebot.ChatMember) error
	RevokeInviteLink(chat telebot.Recipient, link string) (*telebot.ChatInviteLink, error)
	Send(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error)
	SendAlbum(to telebot.Recipient, a telebot.Album, opts ...interface{}) ([]telebot.Message, error)
	SetAdminTitle(chat *telebot.Chat, user *telebot.User, title string) error
	SetCommands(opts ...interface{}) error
	SetCustomEmojiStickerSetThumb(name string, id string) error
	SetDefaultRights(rights telebot.Rights, forChannels bool) error
	SetGameScore(user telebot.Recipient, msg telebot.Editable, score telebot.GameHighScore) (*telebot.Message, error)
	SetGroupDescription(chat *telebot.Chat, description string) error
	SetGroupPermissions(chat *telebot.Chat, perms telebot.Rights) error
	SetGroupPhoto(chat *telebot.Chat, p *telebot.Photo) error
	SetGroupStickerSet(chat *telebot.Chat, setName string) error
	SetGroupTitle(chat *telebot.Chat, title string) error
	SetMenuButton(chat *telebot.User, mb interface{}) error
	SetMyDescription(desc string, language string) error
	SetMyName(name string, language string) error
	SetMyShortDescription(desc string, language string) error
	SetStickerEmojis(sticker string, emojis []string) error
	SetStickerKeywords(sticker string, keywords []string) error
	SetStickerMaskPosition(sticker string, mask telebot.MaskPosition) error
	SetStickerPosition(sticker string, position int) error
	SetStickerSetThumb(of telebot.Recipient, set *telebot.StickerSet) error
	SetStickerSetTitle(s telebot.StickerSet) error
	SetWebhook(w *telebot.Webhook) error
	Ship(query *telebot.ShippingQuery, what ...interface{}) error
	Start()
	StickerSet(name string) (*telebot.StickerSet, error)
	Stop()
	StopLiveLocation(msg telebot.Editable, opts ...interface{}) (*telebot.Message, error)
	StopPoll(msg telebot.Editable, opts ...interface{}) (*telebot.Poll, error)
	TopicIconStickers() ([]telebot.Sticker, error)
	Trigger(endpoint interface{}, c telebot.Context) error
	Unban(chat *telebot.Chat, user *telebot.User, forBanned ...bool) error
	UnbanSenderChat(chat *telebot.Chat, sender telebot.Recipient) error
	UnhideGeneralTopic(chat *telebot.Chat) error
	Unpin(chat telebot.Recipient, messageID ...int) error
	UnpinAll(chat telebot.Recipient) error
	UnpinAllGeneralTopicMessages(chat *telebot.Chat) error
	UnpinAllTopicMessages(chat *telebot.Chat, topic *telebot.Topic) error
	UploadSticker(to telebot.Recipient, format telebot.StickerSetFormat, f telebot.File) (*telebot.File, error)
	Use(middleware ...telebot.MiddlewareFunc)
	UserBoosts(chat telebot.Recipient, user telebot.Recipient) ([]telebot.Boost, error)
	Webhook() (*telebot.Webhook, error)
}
