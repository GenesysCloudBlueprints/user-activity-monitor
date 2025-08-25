package genesys

import "user-activity-monitor/src/apitypes"

type GenesysUser struct {
	ID                  string                                `json:"id"`
	Name                string                                `json:"name"`
	State               string                                `json:"state"`
	Groups              []GenesysGroup                        `json:"groups"`
	Presence            apitypes.PresenceEventBody            `json:"presence"`
	ConversationSummary apitypes.ConversationSummaryEventBody `json:"conversationSummary"`
	Images              []GenesysUserImage                    `json:"images"`
}

func (u *GenesysUser) GetImageThumbnail() string {
	for _, image := range u.Images {
		if image.Resolution == "x48" {
			return image.ImageURI
		}
	}
	return ""
}

type GenesysUserImage struct {
	Resolution string `json:"resolution"`
	ImageURI   string `json:"imageUri"`
}

type genesysUserResponse struct {
	Entities   []GenesysUser `json:"entities"`
	PageSize   int           `json:"pageSize"`
	PageNumber int           `json:"pageNumber"`
	Total      int           `json:"total"`
	PageCount  int           `json:"pageCount"`
}

type GenesysGroup struct {
	ID      string `json:"id"`
	SelfURI string `json:"selfUri"`
}

type GenesysPresence struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	LanguageLabels map[string]string `json:"languageLabels"`
	SystemPresence string            `json:"systemPresence"`
	DivisionID     string            `json:"divisionId"`
	Deactivated    bool              `json:"deactivated"`
	SelfURI        string            `json:"selfUri"`
}

type genesysPresenceResponse struct {
	Entities []GenesysPresence `json:"entities"`
	Total    int               `json:"total"`
}
