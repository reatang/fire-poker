package file_dto

type ImagePreviewInput struct {
	Path string `json:"path"`
}

type ImagePreviewOutput struct {
	DataURL string `json:"dataURL"`
}
