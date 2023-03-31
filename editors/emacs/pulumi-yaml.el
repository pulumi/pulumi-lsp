;;; package --- pulumi-yaml -*- lexical-binding: t; -*-

;;; Commentary:

(require 'yaml-mode)

(defgroup pulumi-yaml ()
  "Reading and writing Pulumi YAML files."
  :group 'languages)

(defcustom pulumi-yaml-server-command "pulumi-lsp"
  "The command used to invoke the Pulumi YAML LSP server."
  :type 'string :group 'pulumi-yaml)

(defcustom pulumi-yaml-server-command-args nil
  "The arg list to pass to `pulumi-yaml-server-command'."
  :type `(repeat string) :group 'pulumi-yaml)

(defcustom pulumi-yaml-lsp-ensure (if (featurep 'lsp-mode)
                                      'lsp-mode
                                    'eglot)
  "If `pulumi-yaml-mode' should eagerly load a LSP host.

If non-nil, `pulumi-yaml' will `require' the relevant LSP mode so
it can inform it about the new server."
  :group 'pulumi-yaml :type '(choice
                              (const nil)
                              (const 'lsp-mode)
                              (const 'eglot)))

(defcustom pulumi-yaml-server-download-arch
  (cond
   ((string-search "arm32" system-configuration) "arm32")
   ((string-search "arm64" system-configuration) "arm64")
   ((string-search "x86_32" system-configuration) "amd32")
   (t "amd64"))
  "The system architecture to download the Pulumi LSP binary for.

Note: automatic downloads are only supported when using `lsp-mode'."
  :group 'pulumi-yaml :type '(choice
                              (const "arm32")
                              (const "arm64")
                              (const "amd32")
                              (const "amd64")))

(defcustom pulumi-yaml-server-download-url
  (format "https://github.com/pulumi/pulumi-lsp/releases/latest/download/pulumi-lsp-%s-%s.gz"
          (pcase system-type
            ('gnu/linux "linux")
            ('darwin "darwin")
            ('windows-nt "windows"))
          pulumi-yaml-server-download-arch)
  "The download path to retrieve the server from.

Note: automatic downloads are only supported when using `lsp-mode'."
  :group 'pulumi-yaml :type 'string)

(define-derived-mode pulumi-yaml-mode yaml-mode "Pulumi YAML"
  "A YAML derivative specifically for writing Pulumi programs in YAML."
  :group 'pulumi-yaml
  (when pulumi-yaml-lsp-ensure
    (require pulumi-yaml-lsp-ensure)))

(add-to-list 'auto-mode-alist (cons (regexp-quote "Pulumi.yaml") 'pulumi-yaml-mode))
(add-to-list 'auto-mode-alist (cons (regexp-quote "Pulumi.yml") 'pulumi-yaml-mode))
(add-to-list 'auto-mode-alist (cons (regexp-quote "Main.yaml") 'pulumi-yaml-mode))

(with-eval-after-load 'lsp-mode
  (require 'lsp-mode)

  (defcustom pulumi-yaml-store-path
    (expand-file-name
     "pulumi-lsp"
     (expand-file-name "pulumi-yaml" lsp-server-install-dir))
    "The path where the server is installed to."
    :group 'pulumi-yaml :type 'string)

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

  (add-to-list 'lsp-language-id-configuration '(pulumi-yaml-mode . "pulumi-lsp")))

(with-eval-after-load 'eglot
  (add-to-list 'eglot-server-programs
               `(pulumi-yaml-mode . ,(cons "pulumi-lsp" pulumi-yaml-server-command-args))))

(provide 'pulumi-yaml)

;;; pulumi-yaml.el ends here
