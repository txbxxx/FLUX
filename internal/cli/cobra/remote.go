package cobra

import (
	"fmt"
	"io"
	"strings"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
	typesRemote "flux/internal/types/remote"
)

// newRemoteCommand creates the remote command group.
func newRemoteCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "remote",
		Short: "管理远端 Git 仓库",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "请指定 remote 操作，例如: fl remote add <url>")
			return errCommandHandled
		},
	}

	command.AddCommand(
		newRemoteAddCommand(deps),
		newRemoteListCommand(deps),
		newRemoteRemoveCommand(deps),
	)

	return command
}

// newRemoteAddCommand creates the remote add subcommand.
func newRemoteAddCommand(deps Dependencies) *spcobra.Command {
	var name string
	var branch string
	var authType string
	var token string
	var sshKey string
	var project string

	command := &spcobra.Command{
		Use:   "add <url>",
		Short: "添加远端仓库",
		Args:  validateExactOneArg("fl" remote add <url>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.AddRemote(cmd.Context(), typesRemote.AddRemoteInput{
				URL:      args[0],
				Name:     name,
				Branch:   branch,
				AuthType: authType,
				Token:    token,
				SSHKey:   sshKey,
				Project:  project,
			})
			if err != nil {
				return err
			}

			printAddedRemote(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&name, "name", "", "", "配置名称（可选，默认从 URL 推导）")
	flags.StringVarP(&branch, "branch", "b", "main", "分支名")
	flags.StringVar(&authType, "auth", "", "认证类型: token / ssh / basic（可选）")
	flags.StringVar(&token, "token", "", "Personal access token")
	flags.StringVar(&sshKey, "ssh-key", "", "SSH 密钥路径")
	flags.StringVarP(&project, "project", "p", "", "绑定到项目名称（可选）")

	return command
}

// newRemoteListCommand creates the remote list subcommand.
func newRemoteListCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "list",
		Short: "查看已配置的远端仓库",
		Aliases: []string{"ls"},
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.ListRemotes(cmd.Context())
			if err != nil {
				return err
			}

			printRemoteList(cmd.OutOrStdout(), result)
			return nil
		},
	}

	return command
}

// newRemoteRemoveCommand creates the remote remove subcommand.
func newRemoteRemoveCommand(deps Dependencies) *spcobra.Command {
	var force bool

	command := &spcobra.Command{
		Use:   "remove <name>",
		Short: "删除远端仓库配置",
		Aliases: []string{"rm"},
		Args:  validateExactOneArg("fl" remote remove <name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			_, err := deps.Workflow.RemoveRemote(cmd.Context(), typesRemote.RemoveRemoteInput{
				Name:  args[0],
				Force: force,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "远端仓库 \"%s\" 已删除\n", args[0])
			return nil
		},
	}

	flags := command.Flags()
	flags.BoolVar(&force, "force", false, "强制删除（即使有项目绑定）")

	_ = force // used via input struct
	return command
}

// printAddedRemote renders the result of adding a remote.
func printAddedRemote(w io.Writer, result *typesRemote.AddRemoteResult) {
	fmt.Fprintln(w, "远端仓库已添加")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "  名称:   %s\n", result.Name)
	fmt.Fprintf(w, "  地址:   %s\n", result.URL)
	fmt.Fprintf(w, "  分支:   %s\n", result.Branch)

	if result.Connected {
		fmt.Fprintf(w, "  连通:   成功\n")
	} else {
		fmt.Fprintf(w, "  连通:   未验证\n")
	}

	if result.Project != "" {
		fmt.Fprintf(w, "  绑定:   %s\n", result.Project)
	}
}

// printRemoteList renders the list of remote configurations.
func printRemoteList(w io.Writer, result *typesRemote.ListRemotesResult) {
	if len(result.Remotes) == 0 {
		fmt.Fprintln(w, "暂无配置的远端仓库。")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "使用 fl remote add <url> 添加远端仓库。")
		return
	}

	fmt.Fprintln(w, "已配置的远端仓库")
	fmt.Fprintln(w)

	for _, r := range result.Remotes {
		defaultMark := ""
		if r.IsDefault {
			defaultMark = " (默认)"
		}
		fmt.Fprintf(w, "  %s%s\n", r.Name, defaultMark)
		fmt.Fprintf(w, "    地址:   %s\n", r.URL)
		fmt.Fprintf(w, "    分支:   %s\n", r.Branch)
		fmt.Fprintf(w, "    状态:   %s\n", r.Status)

		if r.LastSynced != nil {
			fmt.Fprintf(w, "    同步:   %s\n", r.LastSynced.Format("2006-01-02 15:04:05"))
		}
		if len(r.Projects) > 0 {
			fmt.Fprintf(w, "    项目:   %s\n", strings.Join(r.Projects, ", "))
		}
		fmt.Fprintln(w)
	}
}

// printAddedRemote renders the result of adding a remote — keep unused import guard satisfied.
var _ = (*usecase.LocalWorkflow)(nil)
