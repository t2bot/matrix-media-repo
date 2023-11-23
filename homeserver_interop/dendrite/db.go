package dendrite

import (
	"database/sql"
	"errors"

	_ "github.com/lib/pq" // postgres driver
	"github.com/turt2live/matrix-media-repo/homeserver_interop"
)

const selectLocalMedia = "SELECT media_id, media_origin, content_type, file_size_bytes, creation_ts, upload_name, base64hash, user_id FROM mediaapi_media_repository WHERE media_origin = $1;"

type LocalMedia struct {
	homeserver_interop.ImportDbMedia
	MediaId       string
	MediaOrigin   string
	ContentType   string
	FileSizeBytes int64
	CreationTs    int64
	UploadName    string
	Base64Hash    string
	UserId        string
}

type DenDatabase struct {
	homeserver_interop.ImportDb[LocalMedia]
	db         *sql.DB
	statements statements
	origin     string
}

type statements struct {
	selectLocalMedia *sql.Stmt
}

func OpenDatabase(connectionString string, origin string) (*DenDatabase, error) {
	d := DenDatabase{origin: origin}
	var err error

	if d.db, err = sql.Open("postgres", connectionString); err != nil {
		return nil, err
	}

	if d.statements.selectLocalMedia, err = d.db.Prepare(selectLocalMedia); err != nil {
		return nil, err
	}

	return &d, nil
}

func (d *DenDatabase) GetAllMedia() ([]*LocalMedia, error) {
	rows, err := d.statements.selectLocalMedia.Query(d.origin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*LocalMedia{}, nil // no records
		}
		return nil, err
	}

	var results []*LocalMedia
	for rows.Next() {
		var mediaId sql.NullString
		var mediaOrigin sql.NullString
		var contentType sql.NullString
		var sizeBytes sql.NullInt64
		var createdTs sql.NullInt64
		var uploadName sql.NullString
		var b64hash sql.NullString
		var userId sql.NullString
		err = rows.Scan(
			&mediaId,
			&mediaOrigin,
			&contentType,
			&sizeBytes,
			&createdTs,
			&uploadName,
			&b64hash,
			&userId,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, &LocalMedia{
			MediaId:       mediaId.String,
			MediaOrigin:   mediaOrigin.String,
			ContentType:   contentType.String,
			FileSizeBytes: sizeBytes.Int64,
			CreationTs:    createdTs.Int64,
			UploadName:    uploadName.String,
			Base64Hash:    b64hash.String,
			UserId:        userId.String,
		})
	}

	return results, nil
}
