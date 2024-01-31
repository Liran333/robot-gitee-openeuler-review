package main

import (
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/sirupsen/logrus"

	sdk "github.com/opensourceways/go-gitee/gitee"
)

// CheckBranchKeeper will return format and eg: true, true, error
// false, false, err: There will hava error happen, and it will return error
// true, true, nil: The approved commenter in branch keeper list
// false, true, nil: The approved commenter not in branch keeper list
// false, false, nil: The sig-info.yaml is not setting branch keeper
func (bot *robot) CheckBranchKeeper(
	org, repo, commenter string,
	pr *sdk.PullRequestHook,
	cfg *botConfig,
	log *logrus.Entry,
) (bool, bool, error) {
	sigInfoFiles, err := bot.getFiles(
		cfg.BranchKeeper.Owner,
		cfg.BranchKeeper.Repo,
		cfg.BranchKeeper.Branch,
		sigInfoFile,
		log,
	)

	if err != nil {
		return false, false, err
	}

	keepBranches := make(map[string]sets.String)
	fullPath := fmt.Sprintf(`%s/%s/%s`, org, repo, pr.GetBase().GetRef())

	for _, file := range sigInfoFiles.Files {
		if strings.HasPrefix(file.Path.FullPath(), "archived_sigs/") {
			continue
		}

		decodeKeepBranchFile(file.Content, keepBranches, log)
	}

	if value, exist := keepBranches[fullPath]; exist {
		if value.Has(commenter) {
			return true, true, nil
		}

		return false, true, nil
	}

	return false, false, nil
}

func decodeKeepBranchFile(content string, keepBranches map[string]sets.String, log *logrus.Entry) {
	c, err := base64.StdEncoding.DecodeString(content)

	if err != nil {
		log.WithError(err).Error("decode file")

		return
	}

	var m SigInfos

	if err = yaml.Unmarshal(c, &m); err != nil {
		log.WithError(err).Error("code yaml file")

		return
	}

	maintainers := sets.NewString()

	for _, maintainer := range m.Maintainers {
		maintainers.Insert(maintainer.GiteeID)
	}

	for _, branchKeeper := range m.Branches {
		keepers := sets.NewString()

		for _, keeper := range branchKeeper.Keeper {
			keepers.Insert(keeper.GiteeID)
		}

		keepers = keepers.Union(maintainers)

		for _, branch := range branchKeeper.RepoBranch {
			fullPath := fmt.Sprintf(`%s/%s`, branch.Repo, branch.Branch)

			keepBranches[fullPath] = keepers
		}
	}

	return
}
