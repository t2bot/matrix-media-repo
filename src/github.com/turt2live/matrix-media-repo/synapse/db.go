package synapse

import (
	"database/sql"

	_ "github.com/lib/pq" // postgres driver
)

const selectLocalMedia = "SELECT media_id, media_type, media_length, created_ts, upload_name, user_id FROM local_media_repository;"

type SynapseDatabase struct {
	db         *sql.DB
	statements statements
}

type statements struct {
	selectLocalMedia *sql.Stmt
}

func OpenDatabase(connectionString string) (*SynapseDatabase, error) {
	var d SynapseDatabase
	var err error

	if d.db, err = sql.Open("postgres", connectionString); err != nil {
		return nil, err
	}

	if d.statements.selectLocalMedia, err = d.db.Prepare(selectLocalMedia); err != nil {
		return nil, err
	}

	return &d, nil
}

func (d *SynapseDatabase) GetAllMedia() ([]LocalMedia, error) {
	rows, err := d.statements.selectLocalMedia.Query()
	if err != nil {
		if err == sql.ErrNoRows {
			return []LocalMedia{}, nil // no records
		}
		return nil, err
	}

	var results []LocalMedia
	for rows.Next() {
		var mediaId sql.NullString
		var contentType sql.NullString
		var sizeBytes sql.NullInt64
		var createdTs sql.NullInt64
		var uploadName sql.NullString
		var userId sql.NullString
		err = rows.Scan(
			&mediaId,
			&contentType,
			&sizeBytes,
			&createdTs,
			&uploadName,
			&userId,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, LocalMedia{
			MediaId:     mediaId.String,
			ContentType: contentType.String,
			SizeBytes:   sizeBytes.Int64,
			CreatedTs:   createdTs.Int64,
			UploadName:  uploadName.String,
			UserId:      userId.String,
		})
	}

	return results, nil
}
