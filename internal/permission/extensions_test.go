package permission

import "testing"

func TestExtensionsUseNormalWritePermissions(t *testing.T) {
	for _, name := range []string{"generate_image", "generate_video", "generate_music", "export_document", "synthesize_speech", "transcribe_audio"} {
		if !Check(ModePlan, name, "{}").Denied {
			t.Fatalf("plan mode permits %s", name)
		}
		if !Check(ModeDefault, name, "{}").NeedsConfirm {
			t.Fatalf("default mode does not confirm %s", name)
		}
	}
	for _, name := range []string{"repo_map", "git_summary"} {
		if Check(ModePlan, name, "{}").Denied {
			t.Fatalf("plan mode denied %s", name)
		}
	}
}
