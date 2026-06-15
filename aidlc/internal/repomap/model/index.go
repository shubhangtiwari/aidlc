package model

const (
	SchemaVersion = 1

	MapDir         = "docs/map"
	IndexFilename  = "index.json"
	SQLiteFilename = "repo-map.sqlite"

	FilesShard      = "files.jsonl"
	ImportsShard    = "imports.jsonl"
	TestsShard      = "tests.jsonl"
	BlueprintsShard = "blueprints.jsonl"
	DocsShard       = "docs.jsonl"
	ChangesShard    = "changes.jsonl"
)

type ShardFilenames struct {
	Files      string `json:"files"`
	Imports    string `json:"imports"`
	Tests      string `json:"tests"`
	Blueprints string `json:"blueprints"`
	Docs       string `json:"docs"`
	Changes    string `json:"changes"`
}

type IndexMeta struct {
	SchemaVersion int            `json:"schema_version"`
	MapDir        string         `json:"map_dir"`
	IndexFile     string         `json:"index_file"`
	SQLiteFile    string         `json:"sqlite_file"`
	Shards        ShardFilenames `json:"shards"`
}

func DefaultShardFilenames() ShardFilenames {
	return ShardFilenames{
		Files:      FilesShard,
		Imports:    ImportsShard,
		Tests:      TestsShard,
		Blueprints: BlueprintsShard,
		Docs:       DocsShard,
		Changes:    ChangesShard,
	}
}

func DefaultIndexMeta() IndexMeta {
	return IndexMeta{
		SchemaVersion: SchemaVersion,
		MapDir:        MapDir,
		IndexFile:     IndexFilename,
		SQLiteFile:    SQLiteFilename,
		Shards:        DefaultShardFilenames(),
	}
}
