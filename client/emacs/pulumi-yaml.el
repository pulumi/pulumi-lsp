;;; package --- pulumi-yaml

;;; Commentary:

;;; Code:

(require 'yaml-mode)

(defvar pulumi-yaml-no-lsp nil
  "When t, pulumi-yaml will not load or associate a server to lsp.")

(define-derived-mode pulumi-yaml-mode
  yaml-mode "Pulumi YAML"
  "A YAML derivative specifically for writing Pulumi programs in YAML.")

(add-to-list 'auto-mode-alist (cons
                               (regexp-opt '("Pulumi.yaml" "Pulumi.yml" "Main.yaml"))
                               'pulumi-yaml-mode))


(unless pulumi-yaml-no-lsp
  (require 'lsp-mode) ;; Not evaluated at compile time
  (eval-when-compile  ;; To fix compiler warnings
    (declare-function lsp-register-client "lsp-mode")
    (declare-function make-lsp-client "lsp-mode")
    (declare-function lsp-stdio-connection "lsp-mode")
    (defvar lsp-language-id-configuration))
  (lsp-register-client
   (make-lsp-client :new-connection (lsp-stdio-connection "pulumi-lsp")
                    :major-modes '(pulumi-yaml-mode)
                    :server-id 'pulumi-lsp
                    :add-on? t))
  (add-to-list 'lsp-language-id-configuration '(pulumi-yaml-mode . "pulumi-lsp")))

(provide 'pulumi-yaml)

;;; pulumi-yaml ends here
