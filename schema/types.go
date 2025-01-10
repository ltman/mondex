package schema

import (
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Schema represents a MongoDB collection schema
type Schema struct {
	Collection string  `json:"collection"`
	Indexes    []Index `json:"indexes"`
}

// Index represents a MongoDB index configuration
type Index struct {
	Key                     bson.D     `bson:"key"`
	Name                    string     `bson:"name"`
	Background              bool       `bson:"background,omitempty"`
	Unique                  bool       `bson:"unique,omitempty"`
	Sparse                  bool       `bson:"sparse,omitempty"`
	ExpireAfterSeconds      *int32     `bson:"expireAfterSeconds,omitempty"`
	StorageEngine           bson.M     `bson:"storageEngine,omitempty"`
	PartialFilterExpression bson.M     `bson:"partialFilterExpression,omitempty"`
	Collation               *Collation `bson:"collation,omitempty"`
	DefaultLanguage         string     `bson:"default_language,omitempty"`
	LanguageOverride        string     `bson:"language_override,omitempty"`
	TextIndexVersion        int        `bson:"textIndexVersion,omitempty"`
	Weights                 bson.D     `bson:"weights,omitempty"`
	Hidden                  bool       `bson:"hidden,omitempty"`
	WildcardProjection      bson.M     `bson:"wildcardProjection,omitempty"`
}

// Collation specifies language-specific rules for string comparison
type Collation struct {
	Locale          string `bson:"locale"`
	CaseLevel       *bool  `bson:"caseLevel,omitempty"`
	CaseFirst       string `bson:"caseFirst,omitempty"`
	Strength        int    `bson:"strength,omitempty"`
	NumericOrdering *bool  `bson:"numericOrdering,omitempty"`
	Alternate       string `bson:"alternate,omitempty"`
	MaxVariable     string `bson:"maxVariable,omitempty"`
	Normalization   *bool  `bson:"normalization,omitempty"`
	Backwards       *bool  `bson:"backwards,omitempty"`
}

func (i Index) MarshalJSON() ([]byte, error) {
	return bson.MarshalExtJSON(i, false, false)
}

func (i *Index) UnmarshalJSON(bytes []byte) error {
	return bson.UnmarshalExtJSON(bytes, false, &i)
}
