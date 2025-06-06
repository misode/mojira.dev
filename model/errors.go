package model

import "errors"

var ErrIssueRemoved = errors.New("issue was removed")

var ErrIssueNotFound = errors.New("issue not found")
