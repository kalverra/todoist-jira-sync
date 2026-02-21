package syncer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/todoist-jira-sync/config"
	"github.com/kalverra/todoist-jira-sync/jira"
	"github.com/kalverra/todoist-jira-sync/todoist"
)

const (
	commentFromJira    = "[From Jira] "
	commentFromTodoist = "[From Todoist] "
	defaultIssueType   = "Task"
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

// sectionMap maps section IDs to names and vice versa.
type sectionMap struct {
	byID   map[string]string
	byName map[string]string
}

// Run executes a single sync cycle.
func (e *Engine) Run(ctx context.Context) error {
	syncTime := time.Now().UTC()
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
	e.logger.Info().Int("count", len(tasks)).Msg("fetched todoist tasks")

	jql := fmt.Sprintf(
		"project = %s ORDER BY updated DESC",
		e.cfg.JiraProject,
	)
	issues, err := e.jira.SearchIssues(ctx, jql, 100)
	if err != nil {
		return fmt.Errorf("search jira issues: %w", err)
	}
	e.logger.Info().Int("count", len(issues)).Msg("fetched jira issues")

	todoistByJiraKey := make(map[string]*todoist.Task)
	var unlinkedTodoistTasks []*todoist.Task
	for i := range tasks {
		jiraKey := ExtractJiraKey(tasks[i].Description)
		if jiraKey != "" {
			todoistByJiraKey[jiraKey] = &tasks[i]
		} else {
			unlinkedTodoistTasks = append(unlinkedTodoistTasks, &tasks[i])
		}
	}

	jiraByTodoistID := make(map[string]*jira.Issue)
	var unlinkedJiraIssues []*jira.Issue
	for i := range issues {
		todoistID := ExtractTodoistID(issues[i].Fields.Description)
		if todoistID != "" {
			jiraByTodoistID[todoistID] = &issues[i]
		} else {
			unlinkedJiraIssues = append(unlinkedJiraIssues, &issues[i])
		}
	}

	for _, task := range unlinkedTodoistTasks {
		if err := e.createJiraFromTodoist(ctx, task, secMap, syncTime); err != nil {
			e.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("task", task.Content).
				Msg("failed to create jira issue from todoist task")
		}
	}

	for _, issue := range unlinkedJiraIssues {
		if err := e.createTodoistFromJira(
			ctx, issue, project.ID, secMap, syncTime,
		); err != nil {
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
		if err := e.syncLinkedPair(
			ctx, task, issue, project.ID, secMap, syncTime,
		); err != nil {
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
	syncTime time.Time,
) error {
	sectionName := secMap.byID[task.SectionID]
	jiraStatus := e.cfg.JiraStatusForSection(sectionName)

	userDesc := UserDescription(task.Description)

	createReq := jira.CreateIssueRequest{
		Fields: jira.CreateIssueFields{
			Project:     jira.IssueProject{Key: e.cfg.JiraProject},
			Summary:     task.Content,
			Description: userDesc,
			IssueType:   jira.IssueType{Name: defaultIssueType},
		},
	}

	if task.Due != nil && task.Due.Date != "" {
		createReq.Fields.DueDate = task.Due.Date
	}

	created, err := e.jira.CreateIssue(ctx, createReq)
	if err != nil {
		return fmt.Errorf("create jira issue: %w", err)
	}
	e.logger.Info().
		Str("task", task.Content).
		Str("issue_key", created.Key).
		Msg("created jira issue from todoist task")

	jiraDesc := SetTodoistID(userDesc, task.ID, syncTime)
	if err := e.jira.UpdateIssue(ctx, created.Key, jira.UpdateIssueRequest{
		Fields: map[string]any{"description": jiraDesc},
	}); err != nil {
		return fmt.Errorf("update jira issue description: %w", err)
	}

	todoistDesc := SetJiraKey(task.Description, created.Key, syncTime)
	if err := e.updateTodoistDescription(ctx, task.ID, todoistDesc); err != nil {
		return fmt.Errorf("update todoist task description: %w", err)
	}

	if jiraStatus != "" && jiraStatus != "To Do" {
		if err := e.jira.TransitionIssueTo(
			ctx, created.Key, jiraStatus,
		); err != nil {
			e.logger.Warn().Err(err).
				Str("issue_key", created.Key).
				Str("target_status", jiraStatus).
				Msg("failed to transition new issue")
		}
	}

	if task.Checked {
		doneStatus := e.cfg.JiraStatusForSection("Done")
		if err := e.jira.TransitionIssueTo(
			ctx, created.Key, doneStatus,
		); err != nil {
			e.logger.Warn().Err(err).
				Str("issue_key", created.Key).
				Msg("failed to transition issue to done")
		}
	}

	if err := e.syncCommentsToJira(ctx, task.ID, created.Key); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments to new jira issue")
	}

	return nil
}

func (e *Engine) createTodoistFromJira(
	ctx context.Context,
	issue *jira.Issue,
	projectID string,
	secMap sectionMap,
	syncTime time.Time,
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

	userDesc := UserDescription(issue.Fields.Description)
	todoistDesc := SetJiraKey(userDesc, issue.Key, syncTime)

	createReq := todoist.CreateTaskRequest{
		Content:     issue.Fields.Summary,
		Description: todoistDesc,
		ProjectID:   projectID,
		SectionID:   sectionID,
	}
	if issue.Fields.DueDate != "" {
		createReq.DueDate = issue.Fields.DueDate
	}

	task, err := e.todoist.CreateTask(ctx, createReq)
	if err != nil {
		return fmt.Errorf("create todoist task: %w", err)
	}
	e.logger.Info().
		Str("issue_key", issue.Key).
		Str("task_id", task.ID).
		Msg("created todoist task from jira issue")

	jiraDesc := SetTodoistID(issue.Fields.Description, task.ID, syncTime)
	if err := e.jira.UpdateIssue(ctx, issue.Key, jira.UpdateIssueRequest{
		Fields: map[string]any{"description": jiraDesc},
	}); err != nil {
		return fmt.Errorf("update jira description with todoist link: %w", err)
	}

	if err := e.syncCommentsToTodoist(ctx, issue.Key, task.ID); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments to new todoist task")
	}

	return nil
}

func (e *Engine) syncLinkedPair(
	ctx context.Context,
	task *todoist.Task,
	issue *jira.Issue,
	projectID string,
	secMap sectionMap,
	syncTime time.Time,
) error {
	lastSync, hasSyncTime := ExtractSyncedAt(task.Description)
	if !hasSyncTime {
		lastSync, hasSyncTime = ExtractSyncedAt(issue.Fields.Description)
	}

	jiraUpdated, err := issue.ParseUpdated()
	if err != nil {
		return fmt.Errorf("parse jira updated time: %w", err)
	}

	// Without an updated_at on Todoist tasks, we use this heuristic:
	// if Jira was updated after the last sync, Jira is newer; otherwise assume Todoist.
	jiraIsNewer := hasSyncTime && jiraUpdated.After(lastSync)

	if jiraIsNewer {
		e.logger.Debug().
			Str("task_id", task.ID).
			Str("issue_key", issue.Key).
			Msg("jira is newer, syncing jira -> todoist")
		return e.pushJiraToTodoist(
			ctx, task, issue, projectID, secMap, syncTime,
		)
	}

	e.logger.Debug().
		Str("task_id", task.ID).
		Str("issue_key", issue.Key).
		Msg("todoist is newer (or first sync), syncing todoist -> jira")
	return e.pushTodoistToJira(ctx, task, issue, secMap, syncTime)
}

func (e *Engine) pushJiraToTodoist(
	ctx context.Context,
	task *todoist.Task,
	issue *jira.Issue,
	projectID string,
	secMap sectionMap,
	syncTime time.Time,
) error {
	jiraUserDesc := UserDescription(issue.Fields.Description)
	todoistDesc := SetJiraKey(jiraUserDesc, issue.Key, syncTime)
	summary := issue.Fields.Summary

	updateReq := todoist.UpdateTaskRequest{
		Content:     &summary,
		Description: &todoistDesc,
	}
	if issue.Fields.DueDate != "" {
		updateReq.DueDate = &issue.Fields.DueDate
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
				sec, err := e.todoist.CreateSection(
					ctx, projectID, targetSection,
				)
				if err != nil {
					return fmt.Errorf(
						"create todoist section %q: %w",
						targetSection, err,
					)
				}
				targetSectionID = sec.ID
				secMap.byID[sec.ID] = targetSection
				secMap.byName[targetSection] = sec.ID
			}
			if err := e.todoist.MoveTaskToSection(
				ctx, task.ID, targetSectionID,
			); err != nil {
				return fmt.Errorf("move todoist task to section: %w", err)
			}
		}
	}

	jiraDesc := SetTodoistID(
		UserDescription(issue.Fields.Description), task.ID, syncTime,
	)
	if err := e.jira.UpdateIssue(ctx, issue.Key, jira.UpdateIssueRequest{
		Fields: map[string]any{"description": jiraDesc},
	}); err != nil {
		return fmt.Errorf("update jira synced-at: %w", err)
	}

	if err := e.syncCommentsToTodoist(ctx, issue.Key, task.ID); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments jira -> todoist")
	}

	return nil
}

func (e *Engine) pushTodoistToJira(
	ctx context.Context,
	task *todoist.Task,
	issue *jira.Issue,
	secMap sectionMap,
	syncTime time.Time,
) error {
	todoistUserDesc := UserDescription(task.Description)
	jiraDesc := SetTodoistID(todoistUserDesc, task.ID, syncTime)

	fields := map[string]any{
		"summary":     task.Content,
		"description": jiraDesc,
	}
	if task.Due != nil && task.Due.Date != "" {
		fields["duedate"] = task.Due.Date
	}

	if err := e.jira.UpdateIssue(
		ctx, issue.Key, jira.UpdateIssueRequest{Fields: fields},
	); err != nil {
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
			if err := e.jira.TransitionIssueTo(
				ctx, issue.Key, targetJiraStatus,
			); err != nil {
				e.logger.Warn().Err(err).
					Str("issue_key", issue.Key).
					Str("target", targetJiraStatus).
					Msg("failed to transition jira issue")
			}
		}
	}

	todoistDesc := SetJiraKey(
		UserDescription(task.Description), issue.Key, syncTime,
	)
	if err := e.updateTodoistDescription(ctx, task.ID, todoistDesc); err != nil {
		return fmt.Errorf("update todoist synced-at: %w", err)
	}

	if err := e.syncCommentsToJira(ctx, task.ID, issue.Key); err != nil {
		e.logger.Warn().Err(err).
			Msg("failed to sync comments todoist -> jira")
	}

	return nil
}

func (e *Engine) syncCommentsToJira(
	ctx context.Context,
	todoistTaskID, jiraIssueKey string,
) error {
	todoistComments, err := e.todoist.GetComments(ctx, todoistTaskID)
	if err != nil {
		return fmt.Errorf("get todoist comments: %w", err)
	}

	jiraComments, err := e.jira.GetComments(ctx, jiraIssueKey)
	if err != nil {
		return fmt.Errorf("get jira comments: %w", err)
	}

	existingBodies := make(map[string]bool)
	for _, c := range jiraComments {
		existingBodies[c.Body] = true
	}

	for _, tc := range todoistComments {
		if strings.HasPrefix(tc.Content, commentFromJira) {
			continue
		}
		syncedBody := commentFromTodoist + tc.Content
		if existingBodies[syncedBody] {
			continue
		}
		if _, err := e.jira.AddComment(
			ctx, jiraIssueKey, syncedBody,
		); err != nil {
			e.logger.Warn().Err(err).
				Str("issue_key", jiraIssueKey).
				Msg("failed to add comment to jira")
		}
	}

	return nil
}

func (e *Engine) syncCommentsToTodoist(
	ctx context.Context,
	jiraIssueKey, todoistTaskID string,
) error {
	jiraComments, err := e.jira.GetComments(ctx, jiraIssueKey)
	if err != nil {
		return fmt.Errorf("get jira comments: %w", err)
	}

	todoistComments, err := e.todoist.GetComments(ctx, todoistTaskID)
	if err != nil {
		return fmt.Errorf("get todoist comments: %w", err)
	}

	existingContents := make(map[string]bool)
	for _, c := range todoistComments {
		existingContents[c.Content] = true
	}

	for _, jc := range jiraComments {
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

	return nil
}

func (e *Engine) updateTodoistDescription(
	ctx context.Context,
	taskID, description string,
) error {
	_, err := e.todoist.UpdateTask(ctx, taskID, todoist.UpdateTaskRequest{
		Description: &description,
	})
	return err
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

func findIssueByKey(issues []jira.Issue, key string) (*jira.Issue, bool) {
	for i := range issues {
		if issues[i].Key == key {
			return &issues[i], true
		}
	}
	return nil, false
}
