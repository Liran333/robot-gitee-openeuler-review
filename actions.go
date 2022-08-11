package main

import (
	"fmt"
	"strings"

	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
)

const (
	retestCommand     = "/retest"
	removeClaCommand  = "/cla cancel"
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
