package jwt

import jwtv5 "github.com/golang-jwt/jwt/v5"

func Sign(claims Claims, secret string) (string, error) {
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func Parse(rawToken, secret string) (*Claims, error) {
	token, err := jwtv5.ParseWithClaims(rawToken, &Claims{}, func(token *jwtv5.Token) (any, error) {
		return []byte(secret), nil
	}, jwtv5.WithValidMethods([]string{jwtv5.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwtv5.ErrTokenInvalidClaims
	}

	return claims, nil
}
