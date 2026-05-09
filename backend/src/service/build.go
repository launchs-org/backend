package service

import (
	"fmt"
	"launchs/shared/model"
)

var (
	ErrBuildJobNotFound = fmt.Errorf("build job not found")
)

// GetBuildJobLogs はビルドジョブのログを取得します
func GetBuildJobLogs(buildJobID string, ownerID string) (string, error) {
	// ビルドジョブを取得
	job, err := model.GetBuildJobByID(buildJobID)
	if err != nil {
		return "", ErrBuildJobNotFound
	}

	// プロジェクトを取得して所有者チェック
	project, err := model.GetProjectByID(job.ProjectID)
	if err != nil {
		return "", err
	}
	if project.OwnerID != ownerID {
		return "", ErrForbidden
	}

	// ログを取得
	logBytes, err := model.GetBuildJobLog(buildJobID)
	if err != nil {
		return "", err
	}

	return string(logBytes), nil
}
