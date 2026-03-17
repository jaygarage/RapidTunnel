/**
 * @author: Jay
 * @date: 2025/3/13
 * @file: redis_test.go
 * @description:
 */

package redisclient

import (
	"RapidTunnel/utils/logrus"
	"testing"
)

func TestGetRandomSet(t *testing.T) {
	query := "@expiration_time:[1753951280 +inf] @source:{kuaidaili}"
	query = ""
	data, err := SRandMemberProxy(query)
	if err == nil {
		logrus.Infof("%s %v", *data, err)
	}
	logrus.Info(err)
}
