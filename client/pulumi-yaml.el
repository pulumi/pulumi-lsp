;;; package --- pulumi-yaml

;;; Commentary:

;;; Code:

(defvar pulumi-yaml-no-lsp nil
  "When t, pulumi-yaml will not load or associate a server to lsp.")

(define-derived-mode pulumi-yaml-mode
  yaml-mode "Pulumi YAML"
  "A YAML derivative specifically for writing Pulumi programs in YAML.")

(add-to-list 'auto-mode-alist '("Pulumi.yaml" . pulumi-yaml-mode))
(add-to-list 'auto-mode-alist '("Pulumi.yml" . pulumi-yaml-mode))
(add-to-list 'auto-mode-alist '("Main.yaml" . pulumi-yaml-mode))


(unless pulumi-yaml-no-lsp
  (require 'lsp-mode)
  (lsp-register-client
   (make-lsp-client :new-connection (lsp-stdio-connection "pulumi-lsp")
                    :major-modes '(pulumi-yaml-mode)
                    :server-id 'pulumi-lsp
                    :add-on? t))
  (add-to-list 'lsp-language-id-configuration '(pulumi-yaml-mode . "pulumi-lsp")))

(provide 'pulumi-yaml)

;;; pulumi-yaml ends here
