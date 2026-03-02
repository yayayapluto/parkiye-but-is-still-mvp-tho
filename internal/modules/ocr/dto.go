package ocr

// detectPlateRequest is the JSON body sent to the Python OCR API.
type detectPlateRequest struct {
	ImagePath string `json:"image_path"`
}

// detectPlateResponse is the JSON response from the Python OCR API on success.
type detectPlateResponse struct {
	DetectedPlate   string  `json:"detected_plate"`
	Confidence      float64 `json:"confidence"`
	OutputImagePath string  `json:"output_image_path"`
}

// detectPlateErrorResponse is the JSON error body returned by the Python OCR API.
type detectPlateErrorResponse struct {
	Detail string `json:"detail"`
}
