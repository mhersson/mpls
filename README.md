Markdown Preview Language Server
================================

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/mhersson/mpls)](https://goreportcard.com/report/github.com/mhersson/mpls)


Built using [GLSP](https://github.com/tliron/glsp) and [Goldmark](https://github.com/yuin/goldmark),
and heavily inspired by [mdpls](https://github.com/euclio/mdpls)

## Overview

Markdown Preview Language Server (`mpls`) is a language server designed to
enhance your Markdown editing experience. With live preview in the browser,
`mpls` allows you to see your Markdown content rendered in real-time. Whether
you're writing documentation or creating notes, `mpls` provides a seamless and
interactive environment.

Built with terminal editors in mind, such as Vim and Helix, which do not have
built-in Markdown rendering, `mpls` bridges the gap by providing a live preview
feature that works alongside these editors. Additionally, `mpls` is compatible
with any editor that supports the Language Server Protocol (LSP), making it a
versatile tool for Markdown editing across various platforms.

## Features

- Live Preview: Instantly see your Markdown changes reflected in the browser.

### Built with Goldmark

`mpls` is built using [Goldmark](https://github.com/yuin/goldmark), a Markdown
parser written in Go. Goldmark is known for its extensibility and performance,
making it an ideal choice for `mpls`.

#### Goldmark extensions

`mpls` utilizes several of Goldmark's extensions to enhance the Markdown rendering
experience:

- Github Flavored Markdown: Goldmark's built in GFM extension ensures Table,
  Strikethrough, Linkify and TaskList elements are displayed correctly.
- Image Rendering: The [img64](https://github.com/tenkoh/goldmark-img64)
  extension allows for seamless integration of images within your Markdown
  files.
- Math Rendering: The [katex](https://github.com/FurqanSoftware/goldmark-katex)
  extension enables the rendering of LaTeX-style mathematical expressions using
  KaTeX, providing a clear and professional presentation of equations.

### Mermaid

`mpls` supports the display of diagrams and flowcharts by integrating
[Mermaid.js](https://mermaid.js.org/), a powerful JavaScript library for
generating diagrams from text definitions.

## Install

The esiest way to install `mpls` is to download one of the pre-built
release binaries. You can find the latest releases on the [Releases
page](https://github.com/mhersson/mpls/releases).

> `mpls` uses CGO, which complicates cross-compiling. Therefore, for now, there
> are only prebuilt binaries available for Linux/amd64.

1. Download the appropriate binary for your operating system.
2. Copy the downloaded binary to a directory that is in your system's PATH. For example:

   ```bash
   cp mpls /usr/local/bin/
   ```

<details>
<summary>Build From Source</summary>

If you prefer to build from source, follow these steps:

1. **Clone the repository** (if you haven't already):

   ```bash
   git clone https://github.com/mhersson/mpls.git
   cd mpls
   ```

2. **Build the project**:

   You can build the project using the following command:

   ```bash
   make build
   ```

   This will compile the source code and create an executable.

3. **Install the executable**:

   You have two options to install the executable:

   - **Option 1: Copy the executable to your PATH**:

     After building, you can manually copy the executable to a directory that is in your system's PATH. For example:

     ```bash
     cp mpls /usr/local/bin/
     ```

   - **Option 2: Use `make install` if you are using GOPATH**:

     If the GOPATH is in your PATH, you can run:

     ```bash
     make install
     ```

     This will install the executable to your `$GOPATH/bin` directory.
</details>

**Verify the installation**:

   After installation, you can verify that `mpls` is installed correctly by running:

   ```bash
   mpls --version
   ```

   This should display the version of the `mpls` executable.

## Configuration examples

**Helix**

Configured to run alongside marksman.

<details>
<summary>languages.toml</summary>

```toml
[[language]]
auto-format = true
language-servers = ["marksman", "mpls"]
name = "markdown"

[language-server.mpls]
command = "mpls"
```
</details>


**Doom-Emacs with lsp-mode**

<details>
<summary>config.el</summary>

```elisp
(after! markdown-mode
  ;; Auto start
  (add-hook 'markdown-mode-local-vars-hook #'lsp!))

(after! lsp-mode
  (defgroup lsp-mpls nil
    "Settings for the mpls language server client."
    :group 'lsp-mode
    :link '(url-link "https://github.com/mhersson/mpls"))

  (defcustom lsp-mpls-server-command "mpls"
    "The binary (or full path to binary) which executes the server."
    :type 'string
    :group 'lsp-mpls)

  (lsp-register-client
  (make-lsp-client :new-connection (lsp-stdio-connection
                                    (lambda ()
                                      (or (executable-find lsp-mpls-server-command)
                                          (lsp-package-path 'mpls)
                                          "mpls")
                                      ))
                    :activation-fn (lsp-activate-on "markdown")
                    :initialized-fn (lambda (workspace)
                                      (with-lsp-workspace workspace
                                                          (lsp--set-configuration
                                                          (lsp-configuration-section "mpls"))
                                                          ))
                    ;; Priority and add-on? are not needed,
                    ;; but makes mpls work alongside other lsp servers like marksman
                    :priority 1
                    :add-on? t
                    :server-id 'mpls)))

```
</details>