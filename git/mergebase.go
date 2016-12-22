package git

import ()

func MergeBaseOctopus(c *Client, commits []Commitish) (CommitID, error) {
	var bestSoFar Commitish = commits[0]
	for _, commit := range commits[1:] {
		closest, err := NearestCommonParent(c, bestSoFar, commit)
		if err != nil {
			return CommitID{}, err
		}
		bestSoFar = closest
	}
	return bestSoFar.CommitID(c)

}
