package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type A1111Config struct {
	BaseURL string
}

type img2imgRequest struct {
	InitImages        []string `json:"init_images"`
	Prompt            string   `json:"prompt"`
	NegativePrompt    string   `json:"negative_prompt"`
	DenoisingStrength float64  `json:"denoising_strength"`
	Steps             int      `json:"steps"`
	CFGScale          float64  `json:"cfg_scale"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	SamplerName       string   `json:"sampler_name"`
	Seed              int64    `json:"seed"`
}

type txt2imgRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt"`
	Steps          int     `json:"steps"`
	CFGScale       float64 `json:"cfg_scale"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	SamplerName    string  `json:"sampler_name"`
	Seed           int64   `json:"seed"`
}

type img2imgResponse struct {
	Images     []string `json:"images"`
	Parameters struct{} `json:"parameters"`
	Info       string   `json:"info"`
}

type A1111Options struct {
	NegativePrompt    string
	DenoisingStrength float64
	Steps             int
	CFGScale          float64
	Width             int
	Height            int
	SamplerName       string
	Seed              int64
	Model             string
}

// A1111Model represents a model available in A1111
type A1111Model struct {
	Title     string `json:"title"`
	ModelName string `json:"model_name"`
	Hash      string `json:"hash"`
	Sha256    string `json:"sha256"`
	Filename  string `json:"filename"`
	Config    string `json:"config"`
}

// A1111Lora represents a LoRA available in A1111
type A1111Lora struct {
	Name     string                 `json:"name"`
	Alias    string                 `json:"alias"`
	Path     string                 `json:"path"`
	Metadata map[string]interface{} `json:"metadata"`
}

// A1111SetModel sets the active model in A1111
func A1111SetModel(cfg A1111Config, modelName string) error {
	type setModelRequest struct {
		SDModelCheckpoint string `json:"sd_model_checkpoint"`
	}

	body := setModelRequest{
		SDModelCheckpoint: modelName,
	}

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling request: %w", err)
	}

	url := cfg.BaseURL + "/sdapi/v1/options"
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return fmt.Errorf("calling A1111 options API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("A1111 options API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// A1111ListModels fetches available models from A1111
func A1111ListModels(cfg A1111Config) ([]A1111Model, error) {
	url := cfg.BaseURL + "/sdapi/v1/sd-models"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("calling A1111 models API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("A1111 models API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var models []A1111Model
	if err := json.Unmarshal(respBody, &models); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	return models, nil
}

// A1111ListLoras fetches available LoRAs from A1111
// Note: This endpoint may not exist in all A1111 versions (returns 404 if not available)
func A1111ListLoras(cfg A1111Config) ([]A1111Lora, error) {
	url := cfg.BaseURL + "/sdapi/v1/loras"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("calling A1111 loras API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("LoRA listing not supported by this A1111 instance (endpoint /sdapi/v1/loras not found); this feature requires an A1111 version/extension that exposes the LoRA API")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("A1111 loras API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var loras []A1111Lora
	if err := json.Unmarshal(respBody, &loras); err != nil {
		return nil, fmt.Errorf("parsing loras response: %w", err)
	}

	return loras, nil
}

func A1111GenerateImage(cfg A1111Config, initImage []byte, prompt string, opts A1111Options) ([]byte, error) {
	url := cfg.BaseURL + "/sdapi/v1/txt2img"

	var reqBytes []byte
	var err error

	if len(initImage) > 0 {
		b64 := base64.StdEncoding.EncodeToString(initImage)

		body := img2imgRequest{
			InitImages:        []string{b64},
			Prompt:            prompt,
			NegativePrompt:    opts.NegativePrompt,
			DenoisingStrength: opts.DenoisingStrength,
			Steps:             opts.Steps,
			CFGScale:          opts.CFGScale,
			Width:             opts.Width,
			Height:            opts.Height,
			SamplerName:       opts.SamplerName,
			Seed:              opts.Seed,
		}

		reqBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request: %w", err)
		}

		url = cfg.BaseURL + "/sdapi/v1/img2img"
	} else {
		body := txt2imgRequest{
			Prompt:         prompt,
			NegativePrompt: opts.NegativePrompt,
			Steps:          opts.Steps,
			CFGScale:       opts.CFGScale,
			Width:          opts.Width,
			Height:         opts.Height,
			SamplerName:    opts.SamplerName,
			Seed:           opts.Seed,
		}

		reqBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request: %w", err)
		}
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("calling A1111 API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("A1111 API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result img2imgResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("A1111 returned no images")
	}

	decoded, err := base64.StdEncoding.DecodeString(result.Images[0])
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	return decoded, nil
}
