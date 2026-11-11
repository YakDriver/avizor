package services

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `
service "fsx" {
  sdk {
    id = "FSx"
  }
  names {
    provider_name_upper = "FSx"
    human_friendly      = "FSx"
  }
  resource_prefix {
    correct = "aws_fsx_"
  }
  provider_package_correct = "fsx"
  doc_prefix               = ["fsx_"]
}

service "s3" {
  sdk {
    id = "S3"
  }
  names {
    aliases             = ["s3api"]
    provider_name_upper = "S3"
    human_friendly      = "S3 (Simple Storage)"
  }
  resource_prefix {
    actual  = "aws_(canonical_user_id|s3_bucket|s3_object|s3_directory_bucket)"
    correct = "aws_s3_"
  }
  provider_package_correct = "s3"
}

service "s3control" {
  sdk {
    id = "S3 Control"
  }
  names {
    provider_name_upper = "S3Control"
    human_friendly      = "S3 Control"
  }
  resource_prefix {
    correct = "aws_s3control_"
  }
  provider_package_correct = "s3control"
}

service "amp" {
  names {
    provider_name_upper = "AMP"
    human_friendly      = "AMP (Managed Prometheus)"
  }
  resource_prefix {
    actual  = "aws_prometheus_"
    correct = "aws_amp_"
  }
  provider_package_correct = "amp"
}

service "gone" {
  names {
    provider_name_upper = "Gone"
    human_friendly      = "Gone"
  }
  not_implemented = true
}
`

func load(t *testing.T) *Map {
	t.Helper()
	p := filepath.Join(t.TempDir(), "names_data.hcl")
	if err := os.WriteFile(p, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLoad(t *testing.T) {
	t.Parallel()

	m := load(t)
	if m.Len() != 4 {
		t.Errorf("Len = %d, want 4 (not_implemented skipped)", m.Len())
	}
	for name, want := range map[string]string{
		"aws_fsx_file_system":        "fsx",
		"aws_s3control_access_point": "s3control",
		"aws_s3_bucket_policy":       "s3",
		"aws_prometheus_workspace":   "amp",
		"aws_unknown_thing":          "",
	} {
		got := ""
		if s := m.ByTFName(name); s != nil {
			got = s.Package
		}
		if got != want {
			t.Errorf("ByTFName(%s) = %q, want %q", name, got, want)
		}
	}
}

func TestDetectInText(t *testing.T) {
	t.Parallel()

	m := load(t)
	var got []string
	for _, x := range m.DetectInText("Amazon FSx for NetApp ONTAP now supports Amazon S3 Control and Managed Prometheus") {
		got = append(got, x.Service.Package)
	}
	want := []string{"amp", "fsx", "s3", "s3control"}
	if len(got) != len(want) {
		t.Fatalf("DetectInText = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DetectInText = %v, want %v", got, want)
		}
	}
	if got := m.DetectInText("fsx lowercase and s3x"); len(got) != 0 {
		t.Errorf("expected case-sensitive whole-word matches only, got %v", got)
	}
}

// TestLoadProvider parses the real provider file when RADAR_NAMES_DATA points to it.
func TestLoadProvider(t *testing.T) {
	t.Parallel()

	p := os.Getenv("RADAR_NAMES_DATA")
	if p == "" {
		t.Skip("set RADAR_NAMES_DATA to a provider names_data.hcl to run")
	}
	m, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Len() < 200 {
		t.Errorf("only %d services loaded", m.Len())
	}
	if s := m.ByTFName("aws_fsx_s3_access_point_attachment"); s == nil || s.Package != "fsx" {
		t.Errorf("ByTFName(aws_fsx_s3_access_point_attachment) = %v", s)
	}
}
