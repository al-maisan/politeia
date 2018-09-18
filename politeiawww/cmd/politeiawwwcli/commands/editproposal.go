package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/decred/politeia/politeiawww/api/v1"
	"github.com/decred/politeia/util"
)

type EditProposalCmd struct {
	Args struct {
		Token       string   `positional-arg-name:"token"`
		Markdown    string   `positional-arg-name:"markdownFile"`
		Attachments []string `positional-arg-name:"attachmentFiles"`
	} `positional-args:"true" optional:"true"`
	Random bool `long:"random" optional:"true" description:"Generate a random proposal"`
}

func (cmd *EditProposalCmd) Execute(args []string) error {
	token := cmd.Args.Token
	mdFile := cmd.Args.Markdown
	attachmentFiles := cmd.Args.Attachments

	if !cmd.Random && mdFile == "" {
		return fmt.Errorf(ErrorNoProposalFile)
	}

	// Check for user identity
	if cfg.Identity == nil {
		return fmt.Errorf(ErrorNoUserIdentity)
	}

	// Get server public key
	vr, err := c.Version()
	if err != nil {
		return err
	}

	var files []v1.File
	var md []byte
	if cmd.Random {
		// Generate random proposal markdown text
		var b bytes.Buffer
		b.WriteString("This is the proposal title\n")

		for i := 0; i < 10; i++ {
			r, err := util.Random(32)
			if err != nil {
				return err
			}
			b.WriteString(base64.StdEncoding.EncodeToString(r) + "\n")
		}

		md = b.Bytes()
	} else {
		// Read markdown file into memory and convert to type File
		fpath := util.CleanAndExpandPath(mdFile, cfg.HomeDir)

		var err error
		md, err = ioutil.ReadFile(fpath)
		if err != nil {
			return fmt.Errorf("ReadFile %v: %v", fpath, err)
		}
	}

	f := v1.File{
		Name:    "index.md",
		MIME:    http.DetectContentType(md),
		Digest:  hex.EncodeToString(util.Digest(md)),
		Payload: base64.StdEncoding.EncodeToString(md),
	}

	files = append(files, f)

	// Read attachment files into memory and convert to type File
	for _, file := range attachmentFiles {
		path := util.CleanAndExpandPath(file, cfg.HomeDir)
		attachment, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("ReadFile %v: %v", path, err)
		}

		f := v1.File{
			Name:    filepath.Base(file),
			MIME:    http.DetectContentType(attachment),
			Digest:  hex.EncodeToString(util.Digest(attachment)),
			Payload: base64.StdEncoding.EncodeToString(attachment),
		}

		files = append(files, f)
	}

	// Compute merkle root and sign it
	sig, err := SignMerkleRoot(files, cfg.Identity)
	if err != nil {
		return fmt.Errorf("SignMerkleRoot: %v", err)
	}

	// Setup edit proposal request
	ep := &v1.EditProposal{
		Token:     token,
		Files:     files,
		PublicKey: hex.EncodeToString(cfg.Identity.Public.Key[:]),
		Signature: sig,
	}

	// Print request details
	err = Print(ep, cfg.Verbose, cfg.RawJSON)
	if err != nil {
		return err
	}

	// Send request
	epr, err := c.EditProposal(ep)
	if err != nil {
		return err
	}

	// Verify proposal censorship record
	err = VerifyProposal(epr.Proposal, vr.PubKey)
	if err != nil {
		return fmt.Errorf("unable to verify proposal %v: %v",
			epr.Proposal.CensorshipRecord.Token, err)
	}

	// Print response details
	return Print(epr, cfg.Verbose, cfg.RawJSON)
}
