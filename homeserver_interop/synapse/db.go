package synapse

import (
	"database/sql"
	"errors"

	_ "github.com/lib/pq" // postgres driver
	"github.com/turt2live/matrix-media-repo/homeserver_interop"
)

const selectLocalMedia = "SELECT media_id, media_type, media_length, created_ts, upload_name, user_id, url_cache FROM local_media_repository;"

type LocalMedia struct {
	homeserver_interop.ImportDbMedia
	MediaId     string
	ContentType string
	SizeBytes   int64
	CreatedTs   int64
	UploadName  string
	UserId      string
	UrlCache    string
}

type SynDatabase struct {
	homeserver_interop.ImportDb[LocalMedia]
	db         *sql.DB
	statements statements
}

type statements struct {
	selectLocalMedia *sql.Stmt
}

func OpenDatabase(connectionString string) (*SynDatabase, error) {
	var d SynDatabase
	var err error

	if d.db, err = sql.Open("postgres", connectionString); err != nil {
		return nil, err
	}

	if d.statements.selectLocalMedia, err = d.db.Prepare(selectLocalMedia); err != nil {
		return nil, err
	}

	return &d, nil
}

func (d *SynDatabase) GetAllMedia() ([]*LocalMedia, error) {
	rows, err := d.statements.selectLocalMedia.Query()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*LocalMedia{}, nil // no records
		}
		return nil, err
	}

	var results []*LocalMedia
	for rows.Next() {
		var mediaId sql.NullString
		var contentType sql.NullString
		var sizeBytes sql.NullInt64
		var createdTs sql.NullInt64
		var uploadName sql.NullString
		var userId sql.NullString
		var urlCache sql.NullString
		err = rows.Scan(
			&mediaId,
			&contentType,
			&sizeBytes,
			&createdTs,
			&uploadName,
			&userId,
			&urlCache,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, &LocalMedia{
			MediaId:     mediaId.String,
			ContentType: contentType.String,
			SizeBytes:   sizeBytes.Int64,
			CreatedTs:   createdTs.Int64,
			UploadName:  uploadName.String,
			UserId:      userId.String,
			UrlCache:    urlCache.String,
		})
	}

	return results, nil
}
