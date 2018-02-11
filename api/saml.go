package api

// saml gin-gonic implementation
// uses saml/samlsp with parts rewritten to make it work with gin-gonic and gin-jwt

import (
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"

	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// logging
var samlLogger = loggo.GetLogger("saml")

// jwt signing method
var jwtSigningMethod = jwt.SigningMethodHS256

type SAML struct {
	idpMetadataURL    *url.URL
	sp                saml.ServiceProvider
	AllowIDPInitiated bool
	TimeFunc          func() time.Time
}

func randomBytes(n int) []byte {
	rv := make([]byte, n)
	if _, err := saml.RandReader.Read(rv); err != nil {
		panic(err)
	}
	return rv
}

func newSAML(strIdpMetadataURL string, X509KeyPair, keyPEMBlock []byte) (*SAML, error) {
	s := SAML{}
	var err error

	if s.TimeFunc == nil {
		s.TimeFunc = time.Now
	}

	keyPair, err := tls.X509KeyPair(X509KeyPair, keyPEMBlock)
	if err != nil {
		// try to fix AWS paramstore newlines missing
		str := string(keyPEMBlock)
		str = strings.Replace(str, "-----BEGIN PRIVATE KEY----- ", "", -1)
		str = strings.Replace(str, " -----END PRIVATE KEY-----", "", -1)
		str = strings.Replace(str, " ", "\n", -1)
		str = "-----BEGIN PRIVATE KEY-----\n" + str + "\n-----END PRIVATE KEY-----"
		keyPair, err = tls.X509KeyPair(X509KeyPair, []byte(str))
		if err != nil {
			return nil, err
		}
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, err
	}

	s.idpMetadataURL, err = url.Parse(strIdpMetadataURL)
	if err != nil {
		return nil, err
	}

	rootURL, err := url.Parse(util.GetEnv("SAML_ACS_URL", ""))
	if err != nil {
		return nil, err
	}

	samlSP, _ := samlsp.New(samlsp.Options{
		URL:            *rootURL,
		Key:            keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:    keyPair.Leaf,
		IDPMetadataURL: s.idpMetadataURL,
	})

	s.sp = samlSP.ServiceProvider
	s.AllowIDPInitiated = true

	return &s, nil
}

func (s *SAML) getIDPSSOURL() string {
	return s.idpMetadataURL.String()
}

func (s *SAML) getIDPCert() string {
	return "cert"
}

func (s *SAML) samlEnabledHandler(c *gin.Context) {
	if util.GetEnv("SAML_ENABLED", "") == "yes" {
		c.JSON(200, gin.H{
			"saml": "enabled",
		})
	} else {
		c.JSON(200, gin.H{
			"saml": "disabled",
		})
	}
}
func (s *SAML) samlResponseHandler(c *gin.Context) {
	assertion, err := s.sp.ParseResponse(c.Request, s.getPossibleRequestIDs(c))
	if err != nil {
		if parseErr, ok := err.(*saml.InvalidResponseError); ok {
			samlLogger.Errorf("RESPONSE: ===\n%s\n===\nNOW: %s\nERROR: %s", parseErr.Response, parseErr.Now, parseErr.PrivateErr)
		}
		c.JSON(http.StatusForbidden, gin.H{
			"error": http.StatusText(http.StatusForbidden),
		})
		return
	}
	// auth OK, create jwt token
	token := jwt.New(jwtSigningMethod)
	claims := token.Claims.(jwt.MapClaims)
	expire := s.TimeFunc().UTC().Add(time.Hour)
	claims["id"] = assertion.Subject.NameID.Value
	claims["exp"] = expire.Unix()
	claims["orig_iat"] = s.TimeFunc().Unix()

	tokenString, err := token.SignedString([]byte(util.GetEnv("JWT_SECRET", "unsecure secret key 8a045eb")))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	// redirect to UI with jwt token
	c.Redirect(http.StatusFound, util.GetEnv("URL_PREFIX", "")+"/webapp/saml?token="+tokenString)
}

// samlsp/middleware.go adapted for gin gonic
func (s *SAML) samlInitHandler(c *gin.Context) {
	if c.PostForm("SAMLResponse") != "" {
		s.samlResponseHandler(c)
		return
	}
	url := location.Get(c)

	binding := saml.HTTPRedirectBinding
	bindingLocation := s.sp.GetSSOBindingLocation(binding)
	if bindingLocation == "" {
		binding = saml.HTTPPostBinding
		bindingLocation = s.sp.GetSSOBindingLocation(binding)
	}

	req, err := s.sp.MakeAuthenticationRequest(bindingLocation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	relayState := base64.URLEncoding.EncodeToString(randomBytes(42))

	secretBlock := x509.MarshalPKCS1PrivateKey(s.sp.Key)
	state := jwt.New(jwtSigningMethod)
	claims := state.Claims.(jwt.MapClaims)
	claims["id"] = req.ID
	claims["uri"] = url.Scheme + url.Host + url.Path
	signedState, err := state.SignedString(secretBlock)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     fmt.Sprintf("saml_%s", relayState),
		Value:    signedState,
		MaxAge:   int(saml.MaxIssueDelay.Seconds()),
		HttpOnly: true,
		Secure:   url.Scheme == "https",
		Path:     s.sp.AcsURL.Path,
	})

	if binding == saml.HTTPRedirectBinding {
		redirectURL := req.Redirect(relayState)
		c.Redirect(http.StatusFound, redirectURL.String())
		return
	}
	if binding == saml.HTTPPostBinding {
		c.Writer.Header().Add("Content-Security-Policy", ""+
			"default-src; "+
			"script-src 'sha256-AjPdJSbZmeWHnEc5ykvJFay8FTWeTeRbs9dutfZ0HqE='; "+
			"reflected-xss block; referrer no-referrer;")
		c.Writer.Header().Add("Content-type", "text/html")
		c.Writer.Write([]byte(`<!DOCTYPE html><html><body>`))
		c.Writer.Write(req.Post(relayState))
		c.Writer.Write([]byte(`</body></html>`))
		return
	}
	panic("no saml binding found")
}

func (s *SAML) getPossibleRequestIDs(c *gin.Context) []string {
	rv := []string{}
	for _, cookie := range c.Request.Cookies() {
		if !strings.HasPrefix(cookie.Name, "saml_") {
			continue
		}
		samlLogger.Debugf("getPossibleRequestIDs: cookie: %s", cookie.String())

		jwtParser := jwt.Parser{
			ValidMethods: []string{jwtSigningMethod.Name},
		}
		token, err := jwtParser.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
			secretBlock := x509.MarshalPKCS1PrivateKey(s.sp.Key)
			return secretBlock, nil
		})
		if err != nil || !token.Valid {
			samlLogger.Debugf("... invalid token %s", err)
			continue
		}
		claims := token.Claims.(jwt.MapClaims)
		rv = append(rv, claims["id"].(string))
	}

	// If IDP initiated requests are allowed, then we can expect an empty response ID.
	if s.AllowIDPInitiated {
		rv = append(rv, "")
	}

	return rv
}
