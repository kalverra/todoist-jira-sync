package syncer

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	jiraCloud "github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/rs/zerolog"

	"github.com/kalverra/todoist-jira-sync/config"
	"github.com/kalverra/todoist-jira-sync/jira"
	"github.com/kalverra/todoist-jira-sync/todoist"
)

const (
	commentFromJira    = "[From Jira] "
	commentFromTodoist = "[From Todoist] "
	defaultIssueType   = "Story"
	linkLabel          = "jira-sync"
)

// Engine orchestrates bidirectional sync between Todoist and Jira.
type Engine struct {
	todoist *todoist.Client
	jira    *jira.Client
	cfg     *config.Config
	logger  zerolog.Logger
}

// NewEngine creates a new sync engine.
func NewEngine(
	todoistClient *todoist.Client,
	jiraClient *jira.Client,
	cfg *config.Config,
	logger zerolog.Logger,
) *Engine {
	return &Engine{
		todoist: todoistClient,
		jira:    jiraClient,
		cfg:     cfg,
		logger:  logger.With().Str("component", "syncer").Logger(),
	}
}

type sectionMap struct {
	byID   map[string]string
	byName map[string]string
}

// Run executes a single sync cycle.
func (e *Engine) Run(ctx context.Context) error {
	e.logger.Info().Msg("starting sync cycle")

	project, err := e.todoist.FindProjectByName(ctx, e.cfg.TodoistProject)
	if err != nil {
		return fmt.Errorf("find todoist project: %w", err)
	}
	e.logger.Info().
		Str("project_id", project.ID).
		Str("project_name", project.Name).
		Msg("found todoist project")

	sections, err := e.todoist.GetSections(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("get todoist sections: %w", err)
	}
	secMap := buildSectionMap(sections)

	tasks, err := e.todoist.GetTasks(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("get todoist tasks: %w", err)
	}
	e.logger.Debug().Int("count", len(tasks)).Msg("fetched todoist tasks")

	jql := "project = " + e.cfg.JiraProject + " AND assignee = currentUser()"
	if typesJQL := e.cfg.JiraIssueTypesJQL(); typesJQL != "" {
		jql += " AND " + typesJQL
	}
	jql += " ORDER BY updated DESC"
	issues, _, err := e.jira.Issue.Search(ctx, jql, &jiraCloud.SearchOptions{
		MaxResults: 100,
		StartAt:    0,
	})
	if err != nil {
		return fmt.Errorf("search jira issues: %w", err)
	}
	e.logger.Debug().Int("count", len(issues)).Msg("fetched jira issues")

	// Classify Todoist tasks:
	// - Has Jira key prefix in content -> linked
	// - Has jira-sync label but no key -> unlinked, needs Jira issue
	// - No label -> ignored
	todoistByJiraKey := make(map[string]*todoist.Task)
	var unlinkedTodoistTasks []*todoist.Task
	for i := range tasks {
		jiraKey := ExtractJiraKey(tasks[i].Content)
		if jiraKey != "" {
			todoistByJiraKey[jiraKey] = &tasks[i]
		} else if slices.Contains(tasks[i].Labels, linkLabel) {
			unlinkedTodoistTasks = append(unlinkedTodoistTasks, &tasks[i])
		}
	}

	// Classify Jira issues: if already linked to a Todoist task, skip;
	// otherwise create a Todoist task.
	var unlinkedJiraIssues []*jiraCloud.Issue
	for i := range issues {
		if _, linked := todoistByJiraKey[issues[i].Key]; !linked {
			unlinkedJiraIssues = append(unlinkedJiraIssues, &issues[i])
		}
	}

	for _, task := range unlinkedTodoistTasks {
		if err := e.createJiraFromTodoist(ctx, task, secMap); err != nil {
			e.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("task", task.Content).
				Msg("failed to create jira issue from todoist task")
		}
	}

	for _, issue := range unlinkedJiraIssues {
		if err := e.createTodoistFromJira(ctx, issue, project.ID, secMap); err != nil {
			e.logger.Error().Err(err).
				Str("issue_key", issue.Key).
				Str("summary", issue.Fields.Summary).
				Msg("failed to create todoist task from jira issue")
		}
	}

	for jiraKey, task := range todoistByJiraKey {
		issue, ok := findIssueByKey(issues, jiraKey)
		if !ok {
			e.logger.Warn().
				Str("jira_key", jiraKey).
				Str("task_id", task.ID).
				Msg("linked jira issue not found, skipping")
			continue
		}
		if err := e.syncLinkedPair(ctx, task, issue, project.ID, secMap); err != nil {
			e.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("issue_key", issue.Key).
				Msg("failed to sync linked pair")
		}
	}

	e.logger.Info().Msg("sync cycle complete")
	return nil
}

func (e *Engine) createJiraFromTodoist(
	ctx context.Context,
	task *todoist.Task,
	secMap sectionMap,
) error {
	sectionName := secMap.byID[task.SectionID]
	jiraStatus := e.cfg.JiraStatusForSection(sectionName)

	created, _, err := e.jira.Issue.Create(ctx, &jiraCloud.Issue{
		Fields: &jiraCloud.IssueFields{
			Project:     jiraCloud.Project{Key: e.cfg.JiraProject},
			Summary:     task.Content,
			Description: task.Description,
			Type:        jiraCloud.IssueType{Name: defaultIssueType},
		},
	})
	if err != nil {
		return fmt.Errorf("create jira issue: %w", err)
	}
	e.logger.Info().
		Str("task", task.Content).
		Str("issue_key", created.Key).
		Msg("created jira issue from todoist task")

	linkedContent := PrependJiraLink(task.Content, created.Key, e.cfg.JiraURL)
	if _, err := e.todoist.UpdateTask(ctx, task.ID, todoist.UpdateTaskRequest{
		Content: &linkedContent,
	}); err != nil {
		return fmt.Errorf("update todoist task content with jira link: %w", err)
	}

	if jiraStatus != "" && jiraStatus != "To Do" {
		if _, err := e.jira.Issue.DoTransition(ctx, created.Key, jiraStatus); err != nil {
			e.logger.Warn().Err(err).
				Str("issue_key", created.Key).
				Str("target_status", jiraStatus).
				Msg("failed to transition new issue")
		}
	}

	return nil
}

func (e *Engine) createTodoistFromJira(
	ctx context.Context,
	issue *jiraCloud.Issue,
	projectID string,
	secMap sectionMap,
) error {
	statusName := ""
	if issue.Fields.Status != nil {
		statusName = issue.Fields.Status.Name
	}
	sectionName := e.cfg.SectionForJiraStatus(statusName)
	sectionID := secMap.byName[sectionName]

	if sectionID == "" && sectionName != "" {
		sec, err := e.todoist.CreateSection(ctx, projectID, sectionName)
		if err != nil {
			return fmt.Errorf("create todoist section %q: %w", sectionName, err)
		}
		sectionID = sec.ID
		secMap.byID[sec.ID] = sectionName
		secMap.byName[sectionName] = sec.ID
	}

	linkedContent := PrependJiraLink(issue.Fields.Summary, issue.Key, e.cfg.JiraURL)
	createReq := todoist.CreateTaskRequest{
		Content:     linkedContent,
		Description: issue.Fields.Description,
		ProjectID:   projectID,
		SectionID:   sectionID,
		Labels:      []string{linkLabel},
	}
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		createReq.DueDate = duedate.Format("2006-01-02")
	}

	task, err := e.todoist.CreateTask(ctx, createReq)
	if err != nil {
		return fmt.Errorf("create todoist task: %w", err)
	}
	e.logger.Info().
		Str("issue_key", issue.Key).
		Str("task_id", task.ID).
		Msg("created todoist task from jira issue")

	if err := e.syncCommentsToTodoist(ctx, issue, task.ID); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments to new todoist task")
	}

	return nil
}

func (e *Engine) syncLinkedPair(
	ctx context.Context,
	task *todoist.Task,
	issue *jiraCloud.Issue,
	projectID string,
	secMap sectionMap,
) error {
	jiraUpdated := time.Time(issue.Fields.Updated)

	todoistUpdated, err := time.Parse(time.RFC3339Nano, task.UpdatedAt)
	if err != nil {
		e.logger.Warn().Err(err).
			Str("task_id", task.ID).
			Msg("could not parse todoist updated_at, assuming todoist is newer")
		return e.pushTodoistToJira(ctx, task, issue, secMap)
	}

	if jiraUpdated.After(todoistUpdated) {
		e.logger.Debug().
			Str("task_id", task.ID).
			Str("issue_key", issue.Key).
			Msg("jira is newer, syncing jira -> todoist")
		return e.pushJiraToTodoist(ctx, task, issue, projectID, secMap)
	}

	e.logger.Debug().
		Str("task_id", task.ID).
		Str("issue_key", issue.Key).
		Msg("todoist is newer (or same), syncing todoist -> jira")
	return e.pushTodoistToJira(ctx, task, issue, secMap)
}

func (e *Engine) pushJiraToTodoist(
	ctx context.Context,
	task *todoist.Task,
	issue *jiraCloud.Issue,
	projectID string,
	secMap sectionMap,
) error {
	linkedContent := PrependJiraLink(issue.Fields.Summary, issue.Key, e.cfg.JiraURL)
	desc := issue.Fields.Description

	updateReq := todoist.UpdateTaskRequest{
		Content:     &linkedContent,
		Description: &desc,
	}
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		duedateStr := duedate.Format("2006-01-02")
		updateReq.DueDate = &duedateStr
	}

	if _, err := e.todoist.UpdateTask(ctx, task.ID, updateReq); err != nil {
		return fmt.Errorf("update todoist task: %w", err)
	}

	if issue.Fields.Status != nil {
		targetSection := e.cfg.SectionForJiraStatus(issue.Fields.Status.Name)
		currentSection := secMap.byID[task.SectionID]
		if targetSection != currentSection {
			targetSectionID := secMap.byName[targetSection]
			if targetSectionID == "" {
				sec, err := e.todoist.CreateSection(ctx, projectID, targetSection)
				if err != nil {
					return fmt.Errorf("create todoist section %q: %w", targetSection, err)
				}
				targetSectionID = sec.ID
				secMap.byID[sec.ID] = targetSection
				secMap.byName[targetSection] = sec.ID
			}
			if err := e.todoist.MoveTaskToSection(ctx, task.ID, targetSectionID); err != nil {
				return fmt.Errorf("move todoist task to section: %w", err)
			}
		}
	}

	if err := e.syncCommentsToTodoist(ctx, issue, task.ID); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments jira -> todoist")
	}

	return nil
}

func (e *Engine) pushTodoistToJira(
	ctx context.Context,
	task *todoist.Task,
	issue *jiraCloud.Issue,
	secMap sectionMap,
) error {
	summary := StripJiraPrefix(task.Content)

	updateIssue := &jiraCloud.Issue{
		Key: issue.Key,
		Fields: &jiraCloud.IssueFields{
			Summary:     summary,
			Description: task.Description,
		},
	}
	if task.Due != nil && task.Due.Date != "" {
		dueTime, err := time.Parse("2006-01-02", task.Due.Date)
		if err == nil {
			updateIssue.Fields.Duedate = jiraCloud.Date(dueTime)
		}
	}

	if _, _, err := e.jira.Issue.Update(ctx, updateIssue, nil); err != nil {
		return fmt.Errorf("update jira issue: %w", err)
	}

	sectionName := secMap.byID[task.SectionID]
	if sectionName != "" {
		targetJiraStatus := e.cfg.JiraStatusForSection(sectionName)
		currentStatus := ""
		if issue.Fields.Status != nil {
			currentStatus = issue.Fields.Status.Name
		}
		if !strings.EqualFold(targetJiraStatus, currentStatus) {
			if _, err := e.jira.Issue.DoTransition(ctx, issue.Key, targetJiraStatus); err != nil {
				e.logger.Warn().Err(err).
					Str("issue_key", issue.Key).
					Str("target", targetJiraStatus).
					Msg("failed to transition jira issue")
			}
		}
	}

	if err := e.syncCommentsToJira(ctx, task.ID, issue); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments todoist -> jira")
	}

	return nil
}

// syncCommentsToJira syncs comments from a Todoist task to a Jira issue.
// It uses the issue's embedded Comments field to avoid a separate API call.
func (e *Engine) syncCommentsToJira(
	ctx context.Context,
	todoistTaskID string,
	issue *jiraCloud.Issue,
) error {
	todoistComments, err := e.todoist.GetComments(ctx, todoistTaskID)
	if err != nil {
		return fmt.Errorf("get todoist comments: %w", err)
	}

	existingBodies := make(map[string]bool)
	if issue.Fields.Comments != nil {
		for _, c := range issue.Fields.Comments.Comments {
			existingBodies[c.Body] = true
		}
	}

	for _, tc := range todoistComments {
		if strings.HasPrefix(tc.Content, commentFromJira) {
			continue
		}
		syncedBody := commentFromTodoist + tc.Content
		if existingBodies[syncedBody] {
			continue
		}
		comment := &jiraCloud.Comment{Body: syncedBody}
		if _, _, err := e.jira.Issue.AddComment(ctx, issue.Key, comment); err != nil {
			e.logger.Warn().Err(err).
				Str("issue_key", issue.Key).
				Msg("failed to add comment to jira")
		}
	}

	return nil
}

// syncCommentsToTodoist syncs comments from a Jira issue to a Todoist task.
// It uses the issue's embedded Comments field to avoid a separate API call.
func (e *Engine) syncCommentsToTodoist(
	ctx context.Context,
	issue *jiraCloud.Issue,
	todoistTaskID string,
) error {
	todoistComments, err := e.todoist.GetComments(ctx, todoistTaskID)
	if err != nil {
		return fmt.Errorf("get todoist comments: %w", err)
	}

	existingContents := make(map[string]bool)
	for _, c := range todoistComments {
		existingContents[c.Content] = true
	}

	if issue.Fields.Comments != nil {
		for _, jc := range issue.Fields.Comments.Comments {
			if strings.HasPrefix(jc.Body, commentFromTodoist) {
				continue
			}
			syncedContent := commentFromJira + jc.Body
			if existingContents[syncedContent] {
				continue
			}
			if _, err := e.todoist.CreateComment(ctx, todoist.CreateCommentRequest{
				TaskID:  todoistTaskID,
				Content: syncedContent,
			}); err != nil {
				e.logger.Warn().Err(err).
					Str("task_id", todoistTaskID).
					Msg("failed to add comment to todoist")
			}
		}
	}

	return nil
}

func buildSectionMap(sections []todoist.Section) sectionMap {
	sm := sectionMap{
		byID:   make(map[string]string, len(sections)),
		byName: make(map[string]string, len(sections)),
	}
	for _, s := range sections {
		sm.byID[s.ID] = s.Name
		sm.byName[s.Name] = s.ID
	}
	return sm
}

func findIssueByKey(issues []jiraCloud.Issue, key string) (*jiraCloud.Issue, bool) {
	for i := range issues {
		if issues[i].Key == key {
			return &issues[i], true
		}
	}
	return nil, false
}
