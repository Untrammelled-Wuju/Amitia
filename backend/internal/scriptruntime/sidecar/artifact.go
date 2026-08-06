// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

type Artifact struct {
	Kind       Kind
	EntryPath  string
	ArgsPrefix []string
	WorkingDir string
	Source     Source
}

func sidecarRuntimeURI(kind Kind) string {
	switch kind {
	case KindWeChat:
		return "amitia://runtime/sidecar/launcher.mjs"
	case KindQQ:
		return "amitia://runtime/qq-sidecar/launcher.mjs"
	}
	return ""
}

func sidecarFilenames(kind Kind) (launcher, bundle string) {
	switch kind {
	case KindWeChat:
		return "launcher.mjs", "bundle.mjs"
	case KindQQ:
		return "launcher.mjs", "bundle.mjs"
	}
	return "", ""
}

func sidecarSubdir(kind Kind) string {
	switch kind {
	case KindWeChat:
		return "sidecar"
	case KindQQ:
		return "qq-sidecar"
	}
	return ""
}
