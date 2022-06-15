;;; package --- pulumi-yaml -*- lexical-binding: t; -*-

;;; Commentary:

;;; Code:

(require 'yaml-mode)
(require 'lsp-mode)

(define-derived-mode pulumi-yaml-mode
  yaml-mode "Pulumi YAML"
  "A YAML derivative specifically for writing Pulumi programs in YAML.")

(add-to-list 'auto-mode-alist (cons (regexp-quote "Pulumi.yaml") 'pulumi-yaml-mode))
(add-to-list 'auto-mode-alist (cons (regexp-quote "Pulumi.yml") 'pulumi-yaml-mode))
(add-to-list 'auto-mode-alist (cons (regexp-quote "Main.yaml") 'pulumi-yaml-mode))



(defvar pulumi-yaml-server-download-arch
  (cond
   ((string-search "arm32" system-configuration) "arm32")
   ((string-search "arm64" system-configuration) "arm64")
   ((string-search "x86_32" system-configuration) "amd32")
   (t "amd64"))
  "The system architecture to download the Pulumi LSP binary for.")

(defvar pulumi-yaml-server-download-url
  (format "https://github.com/pulumi/pulumi-lsp/releases/latest/download/pulumi-lsp-%s-%s.gz"
          (pcase system-type
            ('gnu/linux "linux")
            ('darwin "darwin")
            ('windows-nt "windows"))
          pulumi-yaml-server-download-arch)
  "The download path to retrieve the server from.")

(defvar pulumi-yaml-store-path
  (expand-file-name
   "pulumi-lsp"
   (expand-file-name "pulumi-yaml" lsp-server-install-dir))
  "The path where the server is installed to.")

(defvar pulumi-yaml-server-command "pulumi-lsp"
  "The command used to invoke the Pulumi YAML LSP server.")

(defvar pulumi-yaml-server-command-args nil
  "The arg list to pass to `pulumi-yaml-server-command'.")

(lsp-dependency
 'pulumi-lsp
 '(:download :url pulumi-yaml-server-download-url
   :decompress :gzip
   :store-path pulumi-yaml-store-path)
 '(:system "pulumi-lsp"))

(lsp-register-client
 (make-lsp-client
  :new-connection (lsp-stdio-connection
                   (lambda ()
                     (cons (or (executable-find pulumi-yaml-server-command)
                               (lsp-package-path 'pulumi-lsp))
                           pulumi-yaml-server-command-args)))
  :major-modes '(pulumi-yaml-mode)
  :server-id 'pulumi-lsp
  :add-on? t
  :download-server-fn (lambda (_client callback error-callback _update?)
                        (lsp-package-ensure
                         'pulumi-lsp
                         (lambda (&rest rest)
                           (lsp-download-path
                            :binary-path pulumi-yaml-store-path
                            :set-executable? t)
                           (apply callback rest))
                         error-callback))))

(add-to-list 'lsp-language-id-configuration '(pulumi-yaml-mode . "pulumi-lsp"))

(provide 'pulumi-yaml)

;;; pulumi-yaml.el ends here
