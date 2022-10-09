package main

import (
	"encoding/base64"
	"fmt"
	"strings"

	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

const (
	retestCommand     = "/retest"
	removeClaCommand  = "/cla cancel"
	rebaseCommand     = "/rebase"
	removeRebase      = "/rebase cancel"
	removeFlattened   = "/flattened cancel"
	baseMergeMethod   = "merge"
	flattenedCommand  = "/flattened"
	removeLabel       = "openeuler-cla/yes"
	msgNotSetReviewer = "**@%s** Thank you for submitting a PullRequest. It is detected that you have not set a reviewer, please set a one."
)

func (bot *robot) removeInvalidCLA(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() ||
		!e.IsPROpen() ||
		!e.IsCreatingCommentEvent() ||
		e.GetComment().GetBody() != removeClaCommand {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetPRNumber()
	commenter := e.GetCommenter()

	hasPermission, err := bot.hasPermission(org, repo, commenter, false, e.GetPullRequest(), cfg, log)
	if err != nil {
		return err
	}

	if !hasPermission {
		return nil
	}

	return bot.cli.RemovePRLabel(org, repo, number, removeLabel)
}

func (bot *robot) handleRebase(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() ||
		!e.IsPROpen() ||
		!e.IsCreatingCommentEvent() ||
		e.GetComment().GetBody() != rebaseCommand {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetPRNumber()
	commenter := e.GetCommenter()

	hasPermission, err := bot.hasPermission(org, repo, commenter, false, e.GetPullRequest(), cfg, log)
	if err != nil {
		return err
	}

	if !hasPermission {
		return nil
	}

	prLabels := e.GetPRLabelSet()
	if _, ok := prLabels["flattened/merge"]; ok {
		return bot.cli.CreatePRComment(org, repo, number,
			"Please use **/flattened cancel** to remove **flattened/merge** label, and try **/rebase** again")
	}

	return bot.cli.AddPRLabel(org, repo, number, "rebase/merge")
}

func (bot *robot) handleFlattened(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() ||
		!e.IsPROpen() ||
		!e.IsCreatingCommentEvent() ||
		e.GetComment().GetBody() != flattenedCommand {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetPRNumber()
	commenter := e.GetCommenter()

	hasPermission, err := bot.hasPermission(org, repo, commenter, false, e.GetPullRequest(), cfg, log)
	if err != nil {
		return err
	}

	if !hasPermission {
		return nil
	}

	prLabels := e.GetPRLabelSet()
	if _, ok := prLabels["rebase/merge"]; ok {
		return bot.cli.CreatePRComment(org, repo, number,
			"Please use **/rebase cancel** to remove **rebase/merge** label, and try **/flattened** again")
	}

	return bot.cli.AddPRLabel(org, repo, number, "flattened/merge")
}

func (bot *robot) doRetest(e *sdk.PullRequestEvent) error {
	if sdk.GetPullRequestAction(e) != sdk.PRActionChangedSourceBranch {
		return nil
	}

	org, repo := e.GetOrgRepo()

	return bot.cli.CreatePRComment(org, repo, e.GetPRNumber(), retestCommand)
}

func (bot *robot) checkReviewer(e *sdk.PullRequestEvent, cfg *botConfig) error {
	if cfg.UnableCheckingReviewerForPR || sdk.GetPullRequestAction(e) != sdk.ActionOpen {
		return nil
	}

	if e.GetPullRequest() != nil && len(e.GetPullRequest().Assignees) > 0 {
		return nil
	}

	org, repo := e.GetOrgRepo()

	return bot.cli.CreatePRComment(
		org, repo, e.GetPRNumber(),
		fmt.Sprintf(msgNotSetReviewer, e.GetPRAuthor()),
	)
}

func (bot *robot) clearLabel(e *sdk.PullRequestEvent) error {
	if sdk.GetPullRequestAction(e) != sdk.PRActionChangedSourceBranch {
		return nil
	}

	labels := e.GetPRLabelSet()
	v := getLGTMLabelsOnPR(labels)

	if labels.Has(approvedLabel) {
		v = append(v, approvedLabel)
	}

	if len(v) > 0 {
		org, repo := e.GetOrgRepo()
		number := e.GetPRNumber()

		if err := bot.cli.RemovePRLabels(org, repo, number, v); err != nil {
			return err
		}

		return bot.cli.CreatePRComment(
			org, repo, number,
			fmt.Sprintf(commentClearLabel, strings.Join(v, ", ")),
		)
	}

	return nil
}

func (bot *robot) genMergeMethod(e *sdk.PullRequestHook, org, repo string, log *logrus.Entry) string {
	mergeMethod := "merge"

	prLabels := e.LabelsToSet()
	sigLabel := ""

	for p := range prLabels {
		if strings.HasSuffix(p, "/merge") {
			if strings.Split(p, "/")[0] == "flattened" {
				return "squash"
			}

			return strings.Split(p, "/")[0]
		}

		if strings.HasPrefix(p, "sig/") {
			sigLabel = p
		}
	}

	if sigLabel == "" {
		return mergeMethod
	}

	sig := strings.Split(sigLabel, "/")[1]
	filePath := fmt.Sprintf("sig/%s/%s/%s/%s", sig, org, repo[0:1], fmt.Sprintf("%s.yaml", repo))

	c, err := bot.cli.GetPathContent("openeuler", "community", filePath, "master")
	if err != nil {
		log.Infof("get repo %s failed, because of %v", fmt.Sprintf(org, repo), err)

		return mergeMethod
	}

	mergeMethod = bot.decodeRepoYaml(c, log)

	return mergeMethod
}

func (bot *robot) removeRebase(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() ||
		!e.IsPROpen() ||
		!e.IsCreatingCommentEvent() ||
		e.GetComment().GetBody() != removeRebase {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetPRNumber()

	commenter := e.GetCommenter()

	hasPermission, err := bot.hasPermission(org, repo, commenter, false, e.GetPullRequest(), cfg, log)
	if err != nil {
		return err
	}

	if !hasPermission {
		return nil
	}

	return bot.cli.RemovePRLabel(org, repo, number, "rebase/merge")
}

func (bot *robot) removeFlattened(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() ||
		!e.IsPROpen() ||
		!e.IsCreatingCommentEvent() ||
		e.GetComment().GetBody() != removeFlattened {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetPRNumber()

	commenter := e.GetCommenter()

	hasPermission, err := bot.hasPermission(org, repo, commenter, false, e.GetPullRequest(), cfg, log)
	if err != nil {
		return err
	}

	if !hasPermission {
		return nil
	}

	return bot.cli.RemovePRLabel(org, repo, number, "flattened/merge")
}

func (bot *robot) decodeRepoYaml(content sdk.Content, log *logrus.Entry) string {
	c, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		log.WithError(err).Error("decode file")

		return baseMergeMethod
	}

	var r Repository
	if err = yaml.Unmarshal(c, &r); err != nil {
		log.WithError(err).Error("code yaml file")

		return baseMergeMethod
	}

	if r.MergeMethod != "" {
		return r.MergeMethod
	}

	return baseMergeMethod
}
