package plex

/* This file contains all the types for Plex Sessions API response. */

// Session is a Plex json struct.
type Session struct {
	User                 User      `json:"User"`
	Player               Player    `json:"Player"`
	TranscodeSession     Transcode `json:"TranscodeSession"`
	Added                string    `json:"addedAt"`
	Art                  string    `json:"art"`
	AudienceRating       string    `json:"audienceRating"`
	AudienceRatingImg    string    `json:"audienceRatingImage"`
	ContentRating        string    `json:"contentRating"`
	Duration             float64   `json:"duration,string"`
	GUID                 string    `json:"guid"`
	GrandparentArt       string    `json:"grandparentArt"`
	GrandparentGUID      string    `json:"grandparentGuid"`
	GrandparentKey       string    `json:"grandparentKey"`
	GrandparentRatingKey string    `json:"grandparentRatingKey"`
	GrandparentTheme     string    `json:"grandparentTheme"`
	GrandparentThumb     string    `json:"grandparentThumb"`
	GrandparentTitle     string    `json:"grandparentTitle"`
	Index                string    `json:"index"`
	Key                  string    `json:"key"`
	LastViewed           int64     `json:"lastViewedAt,string"`
	LibrarySectionID     string    `json:"librarySectionID"`
	LibrarySectionKey    string    `json:"librarySectionKey"`
	LibrarySectionTitle  string    `json:"librarySectionTitle"`
	OriginallyAvailable  string    `json:"originallyAvailableAt"`
	ParentGUID           string    `json:"parentGuid"`
	ParentIndex          string    `json:"parentIndex"`
	ParentKey            string    `json:"parentKey"`
	ParentRatingKey      string    `json:"parentRatingKey"`
	ParentThumb          string    `json:"parentThumb"`
	ParentTitle          string    `json:"parentTitle"`
	PrimaryExtraKey      string    `json:"primaryExtraKey"`
	Rating               string    `json:"rating"`
	RatingImage          string    `json:"ratingImage"`
	RatingKey            string    `json:"ratingKey"`
	SessionKey           string    `json:"sessionKey"`
	Studio               string    `json:"studio"`
	Summary              string    `json:"summary"`
	Thumb                string    `json:"thumb"`
	Title                string    `json:"title"`
	TitleSort            string    `json:"titleSort"`
	Type                 string    `json:"type"`
	Updated              int64     `json:"updatedAt,string"`
	ViewCount            string    `json:"viewCount"`
	ViewOffset           float64   `json:"viewOffset,string"`
	Year                 string    `json:"year"`
	Session              struct {
		Bandwidth int64  `json:"bandwidth"`
		ID        string `json:"id"`
		Location  string `json:"location"`
	} `json:"Session"`
	GuID  []*GUID  `json:"Guid,omitempty"`
	Media []*Media `json:"Media,omitempty"`
	/* Notifiarr does not need these. :shrug:
	Country  []*Country  `json:"Country"`
	Director []*Director `json:"Director"`
	Genre    []*Genre    `json:"Genre"`
	Producer []*Producer `json:"Producer"`
	Role     []*Role     `json:"Role"`
	Similar  []*Similar  `json:"Similar"`
	Writer   []*Writer   `json:"Writer"`
	*/
}

// User is part of a Plex Session.
type User struct {
	ID    string `json:"id"`
	Thumb string `json:"thumb"`
	Title string `json:"title"`
}

// Player is part of a Plex Session.
type Player struct {
	Address     string `json:"address"`
	Device      string `json:"device"`
	MachineID   string `json:"machineIdentifier"`
	Model       string `json:"model"`
	Platform    string `json:"platform"`
	PlatformVer string `json:"platformVersion"`
	Product     string `json:"product"`
	Profile     string `json:"profile"`
	PublicAddr  string `json:"remotePublicAddress"`
	State       string `json:"state"`
	Title       string `json:"title"`
	UserID      int64  `json:"userID"`
	Vendor      string `json:"vendor"`
	Version     string `json:"version"`
	Relayed     bool   `json:"relayed"`
	Local       bool   `json:"local"`
	Secure      bool   `json:"secure"`
}

// Country is part of a Plex Session.
type Country struct {
	Count  string      `json:"count"`
	Filter string      `json:"filter"`
	ID     interface{} `json:"id"`
	Tag    string      `json:"tag"`
}

// Director is part of a Plex Session.
type Director struct {
	Filter string `json:"filter"`
	ID     string `json:"id"`
	Tag    string `json:"tag"`
}

// Genre is part of a Plex Session.
type Genre struct {
	Count  string `json:"count"`
	Filter string `json:"filter"`
	ID     string `json:"id"`
	Tag    string `json:"tag"`
}

// MediaStream is part of a Plex Session.
type MediaStream struct {
	AudioChannelLayout string  `json:"audioChannelLayout,omitempty"`
	BitDepth           string  `json:"bitDepth,omitempty"`
	Bitrate            float64 `json:"bitrate,string"`
	BitrateMode        string  `json:"bitrateMode,omitempty"`
	Channels           string  `json:"channels,omitempty"`
	ChromaLocation     string  `json:"chromaLocation,omitempty"`
	ChromaSubsampling  string  `json:"chromaSubsampling,omitempty"`
	Codec              string  `json:"codec"`
	CodedHeight        string  `json:"codedHeight,omitempty"`
	CodedWidth         string  `json:"codedWidth,omitempty"`
	ColorPrimaries     string  `json:"colorPrimaries,omitempty"`
	ColorTrc           string  `json:"colorTrc,omitempty"`
	Decision           string  `json:"decision"`
	Default            string  `json:"default,omitempty"`
	DisplayTitle       string  `json:"displayTitle"`
	ExtDisplayTitle    string  `json:"extendedDisplayTitle"`
	FrameRate          float64 `json:"frameRate,omitempty,string"`
	HasScalingMatrix   string  `json:"hasScalingMatrix,omitempty"`
	Height             int64   `json:"height,omitempty,string"`
	ID                 string  `json:"id"`
	Index              string  `json:"index"`
	Language           string  `json:"language,omitempty"`
	LanguageCode       string  `json:"languageCode,omitempty"`
	Level              string  `json:"level,omitempty"`
	Location           string  `json:"location"`
	Profile            string  `json:"profile"`
	RefFrames          string  `json:"refFrames,omitempty"`
	SamplingRate       string  `json:"samplingRate,omitempty"`
	ScanType           string  `json:"scanType,omitempty"`
	Selected           string  `json:"selected,omitempty"`
	StreamType         string  `json:"streamType"`
	Width              int64   `json:"width,omitempty,string"`
}

// MediaPart is part of a Plex Session.
type MediaPart struct {
	AudioProfile    string         `json:"audioProfile"`
	Bitrate         int64          `json:"bitrate,string"`
	Container       string         `json:"container"`
	Decision        string         `json:"decision"`
	Duration        float64        `json:"duration,string"`
	File            string         `json:"file"`
	Height          int64          `json:"height,string"`
	ID              string         `json:"id"`
	Indexes         string         `json:"indexes"`
	Key             string         `json:"key"`
	Protocol        string         `json:"protocol"`
	Selected        bool           `json:"selected"`
	Size            string         `json:"size"`
	StreamingOptmzd string         `json:"optimizedForStreaming"`
	VideoProfile    string         `json:"videoProfile"`
	Width           int64          `json:"width,string"`
	Stream          []*MediaStream `json:"Stream"`
}

// Media is part of a Plex Session.
type Media struct {
	AspectRatio     string       `json:"aspectRatio"`
	AudioChannels   int          `json:"audioChannels,string"`
	AudioCodec      string       `json:"audioCodec"`
	AudioProfile    string       `json:"audioProfile"`
	Bitrate         int64        `json:"bitrate,string"`
	Container       string       `json:"container"`
	Duration        float64      `json:"duration,string"`
	Height          int64        `json:"height,string"`
	ID              string       `json:"id"`
	Protocol        string       `json:"protocol"`
	StreamingOptmzd string       `json:"optimizedForStreaming"`
	VideoCodec      string       `json:"videoCodec"`
	VideoFrameRate  string       `json:"videoFrameRate"`
	VideoProfile    string       `json:"videoProfile"`
	VideoResolution string       `json:"videoResolution"`
	Width           int64        `json:"width,string"`
	Selected        bool         `json:"selected"`
	Part            []*MediaPart `json:"Part"`
}

// Producer is part of a Plex Session.
type Producer struct {
	Filter string `json:"filter"`
	ID     string `json:"id"`
	Tag    string `json:"tag"`
}

// Role is part of a Plex Session.
type Role struct {
	Count  string `json:"count,omitempty"`
	Filter string `json:"filter"`
	ID     string `json:"id"`
	Role   string `json:"role"`
	Tag    string `json:"tag"`
	Thumb  string `json:"thumb,omitempty"`
}

// Transcode is part of a Plex Session.
type Transcode struct {
	AudioChannels       int    `json:"audioChannels"`
	AudioCodec          string `json:"audioCodec"`
	AudioDecision       string `json:"audioDecision"`
	Container           string `json:"container"`
	Context             string `json:"context"`
	Duration            int64  `json:"duration"`
	Key                 string `json:"key"`
	MaxOffsetAvailable  string `json:"maxOffsetAvailable"`
	MinOffsetAvailable  string `json:"minOffsetAvailable"`
	Progress            string `json:"progress"`
	Protocol            string `json:"protocol"`
	Remaining           int64  `json:"remaining"`
	Size                int64  `json:"size"`
	SourceAudioCodec    string `json:"sourceAudioCodec"`
	SourceVideoCodec    string `json:"sourceVideoCodec"`
	Speed               string `json:"speed"`
	TimeStamp           string `json:"timeStamp"`
	VideoCodec          string `json:"videoCodec"`
	VideoDecision       string `json:"videoDecision"`
	Throttled           bool   `json:"throttled"`
	Complete            bool   `json:"complete"`
	XcodeHwFullPipeline bool   `json:"transcodeHwFullPipeline"`
	XcodeHwRequested    bool   `json:"transcodeHwRequested"`
}

// Writer is part of a Plex Session.
type Writer struct {
	Filter string `json:"filter"`
	ID     string `json:"id"`
	Tag    string `json:"tag"`
}

// Similar is part of a Plex Session.
type Similar struct {
	Filter string `json:"filter"`
	ID     string `json:"id"`
	Tag    string `json:"tag"`
	Count  string `json:"count,omitempty"`
}
