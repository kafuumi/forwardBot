package push

import "testing"

/*
验证程序
# python 3.10
import time
import hmac
import hashlib
import base64
import urllib.parse

def main():

	secret_enc = secret.encode('utf-8')
	string_to_sign = '{}\n{}'.format(timestamp, secret)
	string_to_sign_enc = string_to_sign.encode('utf-8')
	hmac_code = hmac.new(secret_enc, string_to_sign_enc, digestmod=hashlib.sha256).digest()
	sign = urllib.parse.quote_plus(base64.b64encode(hmac_code))
	print("timestamp=", timestamp)
	print("sign=", sign)
	print("ok=", sign == compare)

if __name__ == '__main__':

	compare = 'gnnHz0Qlxj3%2FumuU6Dqj3M4jqPkHAV%2BgyTBIasxzu%2BA%3D'
	timestamp = '1662361916704'
	secret = 'SEC4e340c6301dcf68db9125250a35d4540a5fcad013589d659b845247e5eb0b1e4'
	main()
*/
func TestSignPusher(t *testing.T) {
	secret := "SEC4e340c6301dcf68db9125250a35d4540a5fcad013589d659b845247e5eb0b1e4"
	timestamp, sign := signPusher(secret)
	t.Logf("timestamp = %s, secret = %s, sign = %s", timestamp, secret, sign)
}
