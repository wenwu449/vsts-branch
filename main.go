package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type secrets struct {
	Username                 string `json:"username"`
	Password                 string `json:"password"`
	Instance                 string `json:"instance"`
	Collection               string `json:"collection"`
	Project                  string `json:"project"`
	Repo                     string `json:"repo"`
	MasterBranch             string `json:"masterBranch"`
	ReleaseBranchPrefix      string `json:"releaseBranchPrefix"`
	VersionPath              string `json:"versionPath"`
	DefinitionPathPrefix     string `json:"definitionPathPrefix"`
	DefinitionName           string `json:"definitionName"`
	OnboardBuildDefinitionID int    `json:"onboardBuildDefinitionId"`
}

type ref struct {
	Name     string `json:"name"`
	ObjectID string `json:"objectId"`
	URL      string `json:"url"`
}

type refs struct {
	Value []ref `json:"value"`
	Count int   `json:"count"`
}

type branch struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
	NewObjectID string `json:"newObjectId"`
}

type commits struct {
	Count int `json:"count"`
	Value []struct {
		CommitID     string `json:"commitId"`
		Comment      string `json:"comment"`
		ChangeCounts struct {
			Add int `json:"Add"`
		} `json:"changeCounts"`
		URL       string `json:"url"`
		RemoteURL string `json:"remoteUrl"`
	} `json:"value"`
}

type refUpdate struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
}

type item struct {
	Path string `json:"path"`
}

type newContent struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

type change struct {
	ChangeType string     `json:"changeType"`
	Item       item       `json:"item"`
	NewContent newContent `json:"newContent"`
}

type pushCommit struct {
	Comment string   `json:"comment"`
	Changes []change `json:"changes"`
}

type push struct {
	RefUpdates []refUpdate  `json:"refUpdates"`
	Commits    []pushCommit `json:"commits"`
}

type version struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type root struct {
	Versions []version `xml:"versions>version"`
}

type definition struct {
	ID int `json:"id"`
}

type buildReq struct {
	Definition   definition `json:"definition"`
	SourceBranch string     `json:"sourceBranch"`
	Parameters   string     `json:"parameters"`
}

type definitions struct {
	Count int `json:"count"`
	Value []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
		URI  string `json:"uri"`
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"value"`
}

type builds struct {
	Count int `json:"count"`
	Value []struct {
		ID            int       `json:"id"`
		URL           string    `json:"url"`
		BuildNumber   string    `json:"buildNumber"`
		URI           string    `json:"uri"`
		SourceBranch  string    `json:"sourceBranch"`
		SourceVersion string    `json:"sourceVersion"`
		Status        string    `json:"status"`
		QueueTime     time.Time `json:"queueTime"`
		Priority      string    `json:"priority"`
		StartTime     time.Time `json:"startTime"`
		FinishTime    time.Time `json:"finishTime"`
		Reason        string    `json:"reason"`
		Result        string    `json:"result"`
		Parameters    string    `json:"parameters"`
		KeepForever   bool      `json:"keepForever"`
	} `json:"value"`
}

type pullRequests struct {
	Value []struct {
		PullRequestID      int       `json:"pullRequestId"`
		CodeReviewID       int       `json:"codeReviewId"`
		Status             string    `json:"status"`
		CreationDate       time.Time `json:"creationDate"`
		Title              string    `json:"title"`
		Description        string    `json:"description"`
		SourceRefName      string    `json:"sourceRefName"`
		TargetRefName      string    `json:"targetRefName"`
		MergeStatus        string    `json:"mergeStatus"`
		MergeID            string    `json:"mergeId"`
		URL                string    `json:"url"`
		SupportsIterations bool      `json:"supportsIterations"`
	} `json:"value"`
	Count int `json:"count"`
}

type pullRequest struct {
	SourceRefName string `json:"sourceRefName"`
	TargetRefName string `json:"targetRefName"`
	Title         string `json:"title"`
	Description   string `json:"description"`
}

type lastMergeSourceCommit struct {
	CommitID string `json:"commitId"`
}

type completionOptions struct {
	DeleteSourceBranch string `json:"deleteSourceBranch"`
	MergeCommitMessage string `json:"mergeCommitMessage"`
	SquashMerge        string `json:"squashMerge"`
	BypassPolicy       string `json:"bypassPolicy"`
}

type patchPullRequest struct {
	Status                string                `json:"status"`
	LastMergeSourceCommit lastMergeSourceCommit `json:"lastMergeSourceCommit"`
	CompletionOptions     completionOptions     `json:"completionOptions"`
}

type diffs struct {
	AllChangesIncluded bool `json:"allChangesIncluded"`
	ChangeCounts       struct {
		Edit int `json:"Edit"`
	} `json:"changeCounts"`
	Changes []struct {
		Item struct {
			ObjectID         string `json:"objectId"`
			OriginalObjectID string `json:"originalObjectId"`
			GitObjectType    string `json:"gitObjectType"`
			CommitID         string `json:"commitId"`
			Path             string `json:"path"`
			IsFolder         bool   `json:"isFolder"`
			URL              string `json:"url"`
		} `json:"item"`
		ChangeType string `json:"changeType"`
	} `json:"changes"`
	CommonCommit string `json:"commonCommit"`
	BaseCommit   string `json:"baseCommit"`
	TargetCommit string `json:"targetCommit"`
	AheadCount   int    `json:"aheadCount"`
	BehindCount  int    `json:"behindCount"`
}

type commit struct {
	CommitID     string `json:"commitId"`
	ChangeCounts struct {
		Edit int `json:"Edit"`
	} `json:"changeCounts"`
	Changes []struct {
		Item struct {
			ObjectID         string `json:"objectId"`
			OriginalObjectID string `json:"originalObjectId"`
			GitObjectType    string `json:"gitObjectType"`
			CommitID         string `json:"commitId"`
			Path             string `json:"path"`
			IsFolder         bool   `json:"isFolder"`
			URL              string `json:"url"`
		} `json:"item"`
		ChangeType string `json:"changeType"`
	} `json:"changes"`
}

var secret = secrets{}

func getRelBranches(client *http.Client, relBranch string) refs {
	getBranchURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/refs/heads/{branch}?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{branch}", relBranch,
		"{version}", "1.0")

	urlString := r.Replace(getBranchURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	relBranches := refs{}

	json.NewDecoder(resp.Body).Decode(&relBranches)

	return relBranches
}

func getMasterBranch(client *http.Client) ref {
	getBranchURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/refs/heads/{branch}?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{branch}", secret.MasterBranch,
		"{version}", "1.0")

	urlString := r.Replace(getBranchURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	masterBranches := refs{}

	json.NewDecoder(resp.Body).Decode(&masterBranches)

	fmt.Printf("master branches: %v\n", masterBranches.Count)

	if masterBranches.Count == 0 {
		panic(fmt.Sprintf("No %v branch found", secret.MasterBranch))
	}

	masterBranch := masterBranches.Value[0]
	for i := range masterBranches.Value {
		if masterBranches.Value[i].Name == secret.MasterBranch {
			masterBranch = masterBranches.Value[i]
			break
		}
	}
	return masterBranch
}

func createBranch(client *http.Client, relBranch string, commitID string) {
	newBranch := branch{
		Name:        fmt.Sprintf("%s/%s", "refs/heads", relBranch),
		OldObjectID: "0000000000000000000000000000000000000000",
		NewObjectID: commitID,
	}

	postBranchURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/refs?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "1.0")

	urlString := r.Replace(postBranchURLTemplate)
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode([]branch{newBranch})

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.Status)
}

func getCommits(client *http.Client, relBranch string, startTime time.Time, endTime time.Time) commits {
	toText, _ := endTime.MarshalText()
	fromText, _ := startTime.MarshalText()
	fmt.Printf("Finding commits from %s to %s...\n", string(fromText), string(toText))

	getCommitsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/commits?api-version={version}&branch={branch}&itemPath={versionPath}&fromDate={fromDateTime}&toDate={toDateTime}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{branch}", relBranch,
		"{versionPath}", secret.VersionPath,
		"{fromDateTime}", string(fromText),
		"{toDateTime}", string(toText),
		"{version}", "1.0")

	urlString := r.Replace(getCommitsURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	commits := commits{}

	json.NewDecoder(resp.Body).Decode(&commits)
	return commits
}

func getBranchVersionXML(client *http.Client, branch string) root {
	getItemURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/items?api-version={version}&versionType={versionType}&version={versionValue}&scopePath={versionPath}&lastProcessedChange=true"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{versionType}", "branch",
		"{versionValue}", branch,
		"{versionPath}", secret.VersionPath,
		"{version}", "1.0")

	urlString := r.Replace(getItemURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bodyText, _ := ioutil.ReadAll(resp.Body)

	versionXML := root{}

	err = xml.Unmarshal(bodyText, &versionXML)
	if err != nil {
		log.Fatal(err)
	}
	return versionXML
}

func getCommitVersionXML(client *http.Client, commitID string) root {
	getItemURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/items?api-version={version}&versionType={versionType}&version={versionValue}&scopePath={versionPath}&lastProcessedChange=true"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{versionType}", "commit",
		"{versionValue}", commitID,
		"{versionPath}", secret.VersionPath,
		"{version}", "1.0")

	urlString := r.Replace(getItemURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bodyText, _ := ioutil.ReadAll(resp.Body)

	versionXML := root{}
	err = xml.Unmarshal(bodyText, &versionXML)
	if err != nil {
		log.Fatal(err)
	}
	return versionXML
}

func resetBuildVersion(client *http.Client, versionXML root, build string, relBranch string, commitID string) {
	versions := strings.Split(versionXML.Versions[0].Value, ".")
	buildNum, err := strconv.Atoi(build)
	if err != nil {
		log.Fatal(err)
	}
	versions[len(versions)-2] = strconv.Itoa(buildNum + 1)
	versions[len(versions)-1] = "0"
	versionXML.Versions[0].Value = strings.Join(versions, ".")
	fmt.Printf("Reset version to: %s\n", versionXML.Versions[0].Value)
	content, err := xml.MarshalIndent(versionXML, "", "  ")

	postPushURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/pushes?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "2.0-preview")

	urlString := r.Replace(postPushURLTemplate)

	versionResetPush := push{
		RefUpdates: []refUpdate{
			{
				Name:        fmt.Sprintf("%s/%s", "refs/heads", relBranch),
				OldObjectID: commitID,
			},
		},
		Commits: []pushCommit{
			{
				Comment: "Reset version for release",
				Changes: []change{
					{
						ChangeType: "edit",
						Item: item{
							Path: secret.VersionPath,
						},
						NewContent: newContent{
							ContentType: "rawtext",
							Content:     string(content),
						},
					},
				},
			},
		},
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(versionResetPush)

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Reset version in %s\n", relBranch)
	fmt.Println(resp.Status)
}

func getBuildDefinitions(client *http.Client, relBranch string) definitions {
	getDefinitionsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/definitions?api-version={version}&path={path}&name={definitionName}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "3.0-preview.2",
		"{path}", fmt.Sprintf("%s\\%s", secret.DefinitionPathPrefix, relBranch),
		"{definitionName}", secret.DefinitionName)

	urlString := r.Replace(getDefinitionsURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(secret.Username, secret.Password)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	defs := definitions{}
	fmt.Println(resp.Status)
	json.NewDecoder(resp.Body).Decode(&defs)

	return defs
}

func onboardBuildDefinition(client *http.Client, relBranch string) {
	postBuildURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/builds?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "2.0")

	urlString := r.Replace(postBuildURLTemplate)

	onboardBuild := buildReq{
		Definition: definition{
			ID: secret.OnboardBuildDefinitionID,
		},
		SourceBranch: fmt.Sprintf("%s/%s", "refs/heads", "master"),
		Parameters:   fmt.Sprintf("{\"GitRepositoryName\":\"Compute-CloudShell\",\"GitBranchName\":\"%s\"}", relBranch),
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(onboardBuild)

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println("Onboarding...")
	fmt.Println(resp.Status)
}

func getBuilds(client *http.Client, defID int) builds {
	getBuildsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/builds?api-version={version}&definitions={defID}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "2.0",
		"{defID}", strconv.Itoa(defID))

	urlString := r.Replace(getBuildsURLTemplate)
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	builds := builds{}
	fmt.Println(resp.Status)
	json.NewDecoder(resp.Body).Decode(&builds)

	return builds
}

func postBuild(client *http.Client, relBranch string, buildDefID int) {
	postBuildURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/builds?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "2.0")

	urlString := r.Replace(postBuildURLTemplate)

	relBuild := buildReq{
		Definition: definition{
			ID: buildDefID,
		},
		SourceBranch: fmt.Sprintf("%s/%s", "refs/heads", relBranch),
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(relBuild)
	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println("Building...")
	fmt.Println(resp.Status)
}

func getPullRequests(client *http.Client, targetBranch string, sourceBranch string) pullRequests {
	getPullRequestsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/pullRequests?api-version={version}&status={status}&sourceRefName={sourceBranch}&targetRefName={targetBranch}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "3.0-preview",
		"{status}", "Active",
		"{sourceBranch}", fmt.Sprintf("%s/%s", "refs/heads", sourceBranch),
		"{targetBranch}", fmt.Sprintf("%s/%s", "refs/heads", targetBranch))

	urlString := r.Replace(getPullRequestsURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	pullRequests := pullRequests{}

	json.NewDecoder(resp.Body).Decode(&pullRequests)

	return pullRequests
}

func submitPullRequest(client *http.Client, sourceBranch string, targetBranch string, title string, description string) {
	postPullRequestURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/pullRequests?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "3.0-preview")

	urlString := r.Replace(postPullRequestURLTemplate)

	pullRequest := pullRequest{
		SourceRefName: fmt.Sprintf("%s/%s", "refs/heads", sourceBranch),
		TargetRefName: fmt.Sprintf("%s/%s", "refs/heads", targetBranch),
		Title:         title,
		Description:   description,
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(pullRequest)
	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Starting PR from %s to %s...\n", sourceBranch, targetBranch)
	fmt.Println(resp.Status)
}

func getDiffsBetweenBranches(client *http.Client, baseBranch string, targetBranch string) diffs {
	getDiffsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/diffs/commits?api-version={version}&targetVersionType=branch&targetVersion={targetBranch}&baseVersionType=branch&baseVersion={baseBranch}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "1.0",
		"{baseBranch}", baseBranch,
		"{targetBranch}", targetBranch)

	urlString := r.Replace(getDiffsURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	diffs := diffs{}

	json.NewDecoder(resp.Body).Decode(&diffs)

	return diffs
}

func getCommit(client *http.Client, commitID string) commit {
	getCommitURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/commits/{commitId}?api-version={version}&changeCount={changeCount}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "1.0",
		"{commitId}", commitID,
		"changeCount", "100")

	urlString := r.Replace(getCommitURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	commit := commit{}

	json.NewDecoder(resp.Body).Decode(&commit)
	return commit
}

func completePullRequest(client *http.Client, pullRequestID int, commitID string, mergeMessage string, bypassPolicy bool, deleteSourceBranch bool) {
	patchPullRequestURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/pullRequests/{pullRequest}?api-version={version}"

	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{pullRequest}", strconv.Itoa(pullRequestID),
		"{version}", "3.0-preview")

	urlString := r.Replace(patchPullRequestURLTemplate)

	patchPullRequest := patchPullRequest{
		Status: "completed",
		LastMergeSourceCommit: lastMergeSourceCommit{
			CommitID: commitID,
		},
		CompletionOptions: completionOptions{
			MergeCommitMessage: mergeMessage,
			SquashMerge:        strconv.FormatBool(true),
			DeleteSourceBranch: strconv.FormatBool(deleteSourceBranch),
			BypassPolicy:       strconv.FormatBool(bypassPolicy),
		},
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(patchPullRequest)
	req, err := http.NewRequest("PATCH", urlString, body)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Complete PR %v...\n", pullRequestID)
	fmt.Println(resp.Status)
}

func updateMasterVersion(done chan bool, build string, relBranch string) {
	client := &http.Client{}

	// check master branch version
	versionXML := getBranchVersionXML(client, secret.MasterBranch)

	if len(versionXML.Versions) != 1 {
		fmt.Printf("Error version xml: %+v\n", versionXML)
		done <- false
		return
	}

	versions := strings.Split(versionXML.Versions[0].Value, ".")
	if build != versions[len(versions)-2] {
		// check PR
		pullRequests := getPullRequests(client, secret.MasterBranch, relBranch)

		if pullRequests.Count == 0 {
			// submit PR
			submitPullRequest(client, relBranch, secret.MasterBranch, "Reset version for release", "Reset version for release")
			time.Sleep(10 * time.Second)
			pullRequests = getPullRequests(client, secret.MasterBranch, relBranch)
		}

		if pullRequests.Count == 0 {
			fmt.Println("Error: No PR found after submit PR.")
			done <- false
			return
		}

		if pullRequests.Count > 1 {
			fmt.Printf("Error: %v PRs found. PR IDs:", pullRequests.Count)
			for _, pr := range pullRequests.Value {
				fmt.Printf("%v, ", pr.PullRequestID)
			}
			done <- false
			return
		}

		// check diff
		diffs := getDiffsBetweenBranches(client, secret.MasterBranch, relBranch)

		if diffs.BehindCount != 0 || diffs.AheadCount != 1 {
			fmt.Printf("Cannot merge PR from %s to %s\n", relBranch, secret.MasterBranch)
			fmt.Printf("Diff between %s and %s, %+v\n", secret.MasterBranch, relBranch, diffs)
			done <- false
			return
		}

		path := ""
		for _, change := range diffs.Changes {
			if !strings.HasPrefix(secret.VersionPath, change.Item.Path) {
				path = change.Item.Path
				break
			}
		}

		if path != "" {
			fmt.Printf("Branch %s has change other than version file: %s\n", relBranch, path)
			done <- false
			return
		}

		// complete PR
		completePullRequest(client, pullRequests.Value[0].PullRequestID, diffs.TargetCommit, pullRequests.Value[0].Title, true, false)

		done <- true
	}
}

func startBuild(done chan bool, relBranch string) {
	client := &http.Client{}

	// check build definition
	defs := getBuildDefinitions(client, relBranch)

	fmt.Printf("Found build definitions: %v\n", defs.Count)

	if defs.Count < 1 {
		// create build definition
		onboardBuildDefinition(client, relBranch)

		i := 0
		for ; i < 10; i++ {
			time.Sleep(30 * time.Second)
			defs = getBuildDefinitions(client, relBranch)
			fmt.Printf("%v\n", defs.Count)
			if defs.Count >= 1 {
				break
			}
		}

		if i >= 10 {
			fmt.Printf("No build definitions after %v seconds...\n", i*30)
			done <- false
			return
		}
	}

	buildDefID := defs.Value[0].ID
	for _, def := range defs.Value {
		if def.Name == secret.DefinitionName {
			buildDefID = def.ID
			break
		}
	}

	fmt.Printf("Build definition ID: %v\n", buildDefID)

	// check build
	builds := getBuilds(client, buildDefID)
	fmt.Printf("Found build: %v\n", builds.Count)
	if builds.Count < 1 {
		// create build
		postBuild(client, relBranch, buildDefID)
	}

	done <- true
}

func main() {
	// read secrets
	file, _ := os.Open("secrets.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&secret)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(secret.Username)

	client := &http.Client{}

	// check branch
	commitID := "0000000000000000000000000000000000000000"

	n := time.Now()
	releaseDate := n.AddDate(0, 0, -1*((int(n.Weekday())+6)%7))
	y, m, d := releaseDate.Date()
	relBranch := fmt.Sprintf("%s%v%02v%02v", secret.ReleaseBranchPrefix, y, int(m), d)
	fmt.Println(relBranch)

	relBranches := getRelBranches(client, relBranch)
	fmt.Printf("release branches: %v\n", relBranches.Count)
	if relBranches.Count > 0 {
		fmt.Println("release branch exists.")
		commitID = relBranches.Value[0].ObjectID
	} else {
		// fork
		masterBranch := getMasterBranch(client)

		createBranch(client, relBranch, masterBranch.ObjectID)
		commitID = masterBranch.ObjectID
	}

	// check version
	versionXML := getBranchVersionXML(client, relBranch)

	if len(versionXML.Versions) != 1 {
		fmt.Printf("Error version xml: %+v\n", versionXML)
		return
	}

	versions := strings.Split(versionXML.Versions[0].Value, ".")
	build := versions[len(versions)-2]

	if versions[len(versions)-1] != "0" {
		fmt.Println(versionXML.Versions[0].Value)
		// check commits
		n = time.Now()
		daysLookBack := 1 + (int(n.Weekday())+6)%7

		commits := getCommits(client, relBranch, n.AddDate(0, 0, -1*daysLookBack), n)
		fmt.Println(commits.Count)

		for _, commit := range commits.Value {
			versionXML := getCommitVersionXML(client, commit.CommitID)

			if len(versionXML.Versions) != 1 {
				fmt.Printf("Found more than one version, commit %s: %+v\n", commit.CommitID, versionXML)
			}

			versions = strings.Split(versionXML.Versions[0].Value, ".")
			if versions[len(versions)-2] != build {
				fmt.Printf("Found version before fork: Commit %s: %+v\n", commit.CommitID, versionXML.Versions[0].Value)

				return
			}
		}
		fmt.Printf("No version reset found in %v days.\n", daysLookBack)

		// reset version
		resetBuildVersion(client, versionXML, build, relBranch, commitID)
	}

	uChan := make(chan bool)
	sChan := make(chan bool)
	go updateMasterVersion(uChan, build, relBranch)
	go startBuild(sChan, relBranch)
	uDone := <-uChan
	sDone := <-sChan
	fmt.Printf("update master version succeeded: %v\n", uDone)
	fmt.Printf("start build succeeded: %v\n", sDone)
}
