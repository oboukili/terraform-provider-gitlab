package gitlab

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	gitlab "github.com/xanzy/go-gitlab"
)

func resourceGitlabTagProtection() *schema.Resource {
	acceptedAccessLevels := make([]string, 0, len(accessLevelID))

	for k := range accessLevelID {
		acceptedAccessLevels = append(acceptedAccessLevels, k)
	}
	return &schema.Resource{
		Create: resourceGitlabTagProtectionCreate,
		Read:   resourceGitlabTagProtectionRead,
		Delete: resourceGitlabTagProtectionDelete,
		Importer: &schema.ResourceImporter{
			State: resourceGitlabTagProtectionImporter,
		},

		Schema: map[string]*schema.Schema{
			"project": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"tag": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
			"create_access_level": {
				Type:         schema.TypeString,
				ValidateFunc: validateValueFunc(acceptedAccessLevels),
				Required:     true,
				ForceNew:     true,
			},
		},
	}
}

func resourceGitlabTagProtectionCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)
	project := d.Get("project").(string)
	tag := gitlab.String(d.Get("tag").(string))
	createAccessLevel := accessLevelID[d.Get("create_access_level").(string)]

	options := &gitlab.ProtectRepositoryTagsOptions{
		Name:              tag,
		CreateAccessLevel: &createAccessLevel,
	}

	log.Printf("[DEBUG] create gitlab tag protection on %v for project %s", options.Name, project)

	tp, _, err := client.ProtectedTags.ProtectRepositoryTags(project, options)
	if err != nil {
		// Remove existing tag protection
		_, err = client.ProtectedTags.UnprotectRepositoryTags(project, *tag)
		if err != nil {
			return err
		}
		// Reprotect tag with updated values
		tp, _, err = client.ProtectedTags.ProtectRepositoryTags(project, options)
		if err != nil {
			return err
		}
	}

	d.SetId(buildTwoPartID(&project, &tp.Name))

	return resourceGitlabTagProtectionRead(d, meta)
}

func resourceGitlabTagProtectionRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)
	project, tag, err := projectAndTagFromID(d.Id())
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] read gitlab tag protection for project %s, tag %s", project, tag)

	pt, _, err := client.ProtectedTags.GetProtectedTag(project, tag)
	if err != nil {
		if is404(err) {
			log.Printf("[DEBUG] gitlab tag protection not found %s/%s", project, tag)
			d.SetId("")
			return nil
		}
		return err
	}

	if err := d.Set("project", project); err != nil {
		return err
	}
	if err := d.Set("tag", pt.Name); err != nil {
		return err
	}
	if v, ok := accessLevelValueToName[pt.CreateAccessLevels[0].AccessLevel]; ok {
		if err := d.Set("create_access_level", v); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unknown access level: %d", pt.CreateAccessLevels[0].AccessLevel)
	}

	d.SetId(buildTwoPartID(&project, &pt.Name))

	return nil
}

func resourceGitlabTagProtectionDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)
	project := d.Get("project").(string)
	tag := d.Get("tag").(string)

	log.Printf("[DEBUG] Delete gitlab protected tag %s for project %s", tag, project)

	_, err := client.ProtectedTags.UnprotectRepositoryTags(project, tag)
	return err
}

func projectAndTagFromID(id string) (string, string, error) {
	project, tag, err := parseTwoPartID(id)

	if err != nil {
		log.Printf("[WARN] cannot get group member id from input: %v", id)
	}
	return project, tag, err
}

func resourceGitlabTagProtectionImporter(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	s := strings.Split(d.Id(), ":")
	if len(s) != 2 {
		d.SetId("")
		return nil, fmt.Errorf("invalid tag protection import format; expected '{project_id}:{tag_name}'")
	}
	project, tag := s[0], s[1]

	d.SetId(buildTwoPartID(&project, &tag))
	if err := d.Set("project", project); err != nil {
		return nil, err
	}
	if err := d.Set("tag", tag); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
