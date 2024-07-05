package schema

import (
	"go.mongodb.org/mongo-driver/bson"
)

// Schema represents a MongoDB collection schema
type Schema struct {
	Collection string  `json:"collection"`
	Indexes    []Index `json:"indexes"`
}

type Index struct {
	Key              bson.M `json:"key"`
	Name             string `json:"name"`
	Version          int    `json:"v"`
	Background       bool   `json:"background,omitempty"`
	Unique           bool   `json:"unique,omitempty"`
	DefaultLanguage  string `json:"default_language,omitempty"`
	LanguageOverride string `json:"language_override,omitempty"`
	TextIndexVersion int    `json:"textIndexVersion,omitempty"`
	Weights          bson.M `json:"weights,omitempty"`
}
