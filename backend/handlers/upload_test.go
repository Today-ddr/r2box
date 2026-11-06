package handlers

import (
	"testing"

	"r2box/models"
)

func TestNormalizeExpiresIn(t *testing.T) {
	cases := map[int]int{
		-30: -30,
		1:   1,
		3:   3,
		7:   7,
		30:  30,
		0:   7,
		2:   7,
		999: 7,
	}

	for input, want := range cases {
		if got := normalizeExpiresIn(input); got != want {
			t.Errorf("normalizeExpiresIn(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestCalcPartSize(t *testing.T) {
	const mib = 1024 * 1024

	// 小文件使用默认分片
	if got := calcPartSize(100 * mib); got != 20*mib {
		t.Errorf("小文件分片 = %d, want %d", got, 20*mib)
	}

	// 超大文件需要放大分片，保证不超过 9000 片
	size := int64(200) * 1024 * mib // 200GB
	partSize := calcPartSize(size)
	if parts := (size + partSize - 1) / partSize; parts > 9000 {
		t.Errorf("分片数 = %d, 超过上限", parts)
	}
	if partSize <= 20*mib {
		t.Errorf("超大文件分片 = %d, 应大于 20MiB", partSize)
	}
}

func TestDownloadPath(t *testing.T) {
	if got := DownloadPath("abc"); got != "/api/files/abc/download" {
		t.Errorf("DownloadPath = %q", got)
	}
}

func TestResolveUploadID(t *testing.T) {
	cases := []struct {
		name      string
		stored    string
		requested string
		want      string
		wantOK    bool
	}{
		{"都为空", "", "", "", false},
		{"只有请求体", "", "req-1", "req-1", true},
		{"只有库", "db-1", "", "db-1", true},
		{"两者一致", "db-1", "db-1", "db-1", true},
		{"两者冲突", "db-1", "req-1", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := &models.File{UploadID: c.stored}
			got, ok := resolveUploadID(file, c.requested)
			if ok != c.wantOK || got != c.want {
				t.Errorf("resolveUploadID = (%q, %v), want (%q, %v)", got, ok, c.want, c.wantOK)
			}
		})
	}
}
