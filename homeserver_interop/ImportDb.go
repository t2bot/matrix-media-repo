package homeserver_interop

type ImportDbMedia interface{}

type ImportDb[M ImportDbMedia] interface {
	GetAllMedia() ([]*M, error)
}
