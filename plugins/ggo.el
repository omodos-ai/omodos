;; Copyright 2026 Peter J. Braam
;;
;; Licensed under the Apache License, Version 2.0 (the "License");
;; you may not use this file except in compliance with the License.
;; You may obtain a copy of the License at
;;
;;     http://www.apache.org/licenses/LICENSE-2.0
;;
;; Unless required by applicable law or agreed to in writing, software
;; distributed under the License is distributed on an "AS IS" BASIS,
;; WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
;; See the License for the specific language governing permissions and
;; limitations under the License.

;;; ggo.el --- Emacs integration for the ggo AI CLI  -*- lexical-binding: t; -*-

;;; Commentary:
;; Provides IPC communication with the ggo Go daemon.

;;; Code:

(require 'ansi-color)

(defun ggo--config-dir ()
  "Return the base ggo configuration directory, respecting GGO_CONFIG_DIR."
  (let ((env-dir (getenv "GGO_CONFIG_DIR")))
    (if (and env-dir (not (string-empty-p env-dir)))
        (expand-file-name env-dir)
      (expand-file-name "~/.ggo"))))

(defun ggo--get-active-project ()
  (let ((config-file (expand-file-name "config.yaml" (ggo--config-dir))))
    (if (file-exists-p config-file)
        (with-temp-buffer
          (insert-file-contents config-file)
          (goto-char (point-min))
          (if (re-search-forward "^last_project:[ \t]*\\(.*?\\)$" nil t)
              (match-string 1)
            (error "No last_project found in ggo config.")))
      (error "ggo config not found at %s" config-file))))

(defun ggo--get-pipe (pipe-name)
  (let* ((proj (ggo--get-active-project))
         (pipe-dir (expand-file-name (format "ggoai-%s/run" proj) (ggo--config-dir))))
    (expand-file-name pipe-name pipe-dir)))

(defvar-local ggo-response-process nil)
(defvar-local ggo-prompt-counter 1)

;; ... [Skipping down to ggo-new-chat] ...

(defun ggo-new-chat ()
  (interactive)
  (let* ((proj (ggo--get-active-project))
         ;; FIX: Using ggo--config-dir dynamically
         (chat-file (expand-file-name (format "ggoai-%s/current-chat.md" proj) (ggo--config-dir))))
    (find-file chat-file)
    (unless (eq major-mode 'ggo-chat-mode)
      (ggo-chat-mode))
    (when (= (buffer-size) 0)
      (let ((ts (format-time-string "%Y-%m-%d %H:%M:%S")))
        (insert (format "# ggo Chat\n*Project: %s*\n*Started: %s*\n\n---\n" proj ts))))
    (when (= (point) (point-max))
      (ggo-insert-prompt))
    (message "Type a prompt or /command, then press C-c C-c (or Return at end of line).")))

(defun ggo-start-listener ()
  (interactive)
  (let* ((pipe (ggo--get-pipe "response.pipe"))
         (buf (current-buffer))
         ;; MAGIC FIX: Force pty to prevent output buffering!
         (process-connection-type t)) 
    (unless (file-exists-p pipe)
      (user-error "ggo daemon is not running!"))
    
    (when (process-live-p ggo-response-process)
      (set-process-query-on-exit-flag ggo-response-process nil)
      (delete-process ggo-response-process))
    
    (setq ggo-response-process
          (make-process
           :name "ggo-response-listener"
           :buffer buf
           ;; FIX: Use 'cat -u' for unbuffered reads across all OSes and batch modes
           :command (list "cat" "-u" pipe)
           :sentinel (lambda (proc event) nil) 
           :filter (lambda (proc string)
                     (when (buffer-live-p (process-buffer proc))
                       (with-current-buffer (process-buffer proc)
                         (let ((moving (= (point) (point-max)))
                               ;; Process daemon ANSI colors
                               (clean-str (ansi-color-apply string)))
                           (save-excursion
                             (goto-char (point-max))
                             (insert clean-str))
                           (when moving (goto-char (point-max)))))))))))

(defun ggo-insert-prompt ()
  "Insert a bash-like prompt at the bottom of the buffer."
  (interactive)
  (goto-char (point-max))
  (unless (bolp) (insert "\n"))
  (let* ((proj (ggo--get-active-project))
         (ts (format-time-string "%H:%M:%S"))
         ;; Use markdown bold so it looks clean
         (prompt-str (format "\n**%s:%s [%d]>** " proj ts ggo-prompt-counter)))
    (insert prompt-str)
    (setq ggo-prompt-counter (1+ ggo-prompt-counter))))

(defun ggo-get-current-prompt-text ()
  (save-excursion
    (let ((end (point-max)))
      ;; Search backwards for the prompt marker
      (if (re-search-backward "\\*\\*.*?>\\*\\*[ \t]*" nil t)
          (progn
            (goto-char (match-end 0))
            (string-trim (buffer-substring-no-properties (point) end)))
        ""))))

(defun ggo-send-prompt ()
  (interactive)
  (let ((input-text (ggo-get-current-prompt-text)))
    (if (string-empty-p input-text)
        (message "Input is empty!")
      
      (goto-char (point-max))
      (insert "\n") ;; Let the daemon provide the response formatting
      
      (if (string-prefix-p "/" input-text)
          (let ((cmd-string (substring input-text 1))
                (pipe (ggo--get-pipe "control.pipe")))
            (write-region (concat cmd-string "\n") nil pipe 'append)
            (message "Command sent."))
        
        (let ((pipe (ggo--get-pipe "prompt.pipe")))
          (write-region (concat input-text "\n") nil pipe 'append)
          (message "Prompt sent."))))))

(defun ggo-control-send (cmd-string)
  (let ((pipe (ggo--get-pipe "control.pipe")))
    (write-region (concat cmd-string "\n") nil pipe 'append)))

(defun ggo-write-files () (interactive) (ggo-control-send "WF"))

(defun ggo-abort-and-prompt ()
  "Send ABORT signal to daemon and instantly drop a new prompt."
  (interactive)
  (ggo-control-send "ABORT")
  (ggo-insert-prompt))

(define-derived-mode ggo-chat-mode markdown-mode "ggo-chat"
  (ggo-start-listener)
  (add-hook 'kill-buffer-hook 
            (lambda ()
              (when (process-live-p ggo-response-process)
                (delete-process ggo-response-process)))
            nil t))

(define-key ggo-chat-mode-map (kbd "C-c C-c") 'ggo-send-prompt)
(define-key ggo-chat-mode-map (kbd "C-c C-w") 'ggo-write-files)
(define-key ggo-chat-mode-map (kbd "C-c C-k") 'ggo-abort-and-prompt)
(define-key ggo-chat-mode-map (kbd "RET")
  (lambda () 
    (interactive) 
    (if (and (= (point) (point-max)) (not (string-empty-p (ggo-get-current-prompt-text))))
        (ggo-send-prompt)
      (newline))))

(defun ggo-new-chat ()
  (interactive)
  (let* ((proj (ggo--get-active-project))
         (chat-file (expand-file-name (format "~/.ggo/ggoai-%s/current-chat.md" proj))))
    (find-file chat-file)
    (unless (eq major-mode 'ggo-chat-mode)
      (ggo-chat-mode))
    (when (= (buffer-size) 0)
      (let ((ts (format-time-string "%Y-%m-%d %H:%M:%S")))
        (insert (format "# ggo Chat\n*Project: %s*\n*Started: %s*\n\n---\n" proj ts))))
    (when (= (point) (point-max))
      (ggo-insert-prompt))
    (message "Type a prompt or /command, then press C-c C-c (or Return at end of line).")))

(defun ggo-jump-to-chat ()
  (interactive)
  (let* ((has-region (use-region-p))
         (selected-text (when has-region (buffer-substring-no-properties (region-beginning) (region-end))))
         (filename (when buffer-file-name (file-name-nondirectory buffer-file-name))))
    
    (if has-region
        (progn
          (setq ggo--target-beg (set-marker (make-marker) (region-beginning)))
          (setq ggo--target-end (set-marker (make-marker) (region-end)))
          (deactivate-mark))
      (progn
        (setq ggo--target-beg (set-marker (make-marker) (point-min)))
        (setq ggo--target-end (set-marker (make-marker) (point-max)))))
    
    (ggo-new-chat)
    
    (when (and has-region filename)
      (goto-char (point-max))
      (insert (format "\nRegarding `%s`:\n```text\n%s\n```\n" filename selected-text)))
    
    (ggo-insert-prompt)
    (message "Ready to prompt!")))

(defun ggo-ediff-clipboard ()
  (interactive)
  (let* ((clip-text (string-trim (shell-command-to-string "pbpaste")))
         (target-buf (current-buffer))
         (m-beg (if (and (boundp 'ggo--target-beg) (markerp ggo--target-beg))
                    ggo--target-beg
                  (set-marker (make-marker) (if (use-region-p) (region-beginning) (point-min)))))
         (m-end (if (and (boundp 'ggo--target-end) (markerp ggo--target-end))
                    ggo--target-end
                  (set-marker (make-marker) (if (use-region-p) (region-end) (point-max)))))
         (orig-text (buffer-substring-no-properties m-beg m-end))
         (orig-buf (generate-new-buffer "*ggo-original*"))
         (clip-buf (generate-new-buffer "*ggo-clipboard*")))
    
    (if (string-empty-p clip-text)
        (user-error "Clipboard is empty! Did the LLM output a code block?")
      (with-current-buffer orig-buf
        (insert orig-text)
        (funcall (buffer-local-value 'major-mode target-buf)))
      (with-current-buffer clip-buf
        (insert clip-text)
        (funcall (buffer-local-value 'major-mode target-buf)))
      (when (use-region-p) (deactivate-mark))
      (ediff-buffers orig-buf clip-buf
                     `((lambda ()
                         (add-hook 'ediff-quit-hook
                                   (lambda ()
                                     (let ((final-text (with-current-buffer ,orig-buf (buffer-string))))
                                       (with-current-buffer ,target-buf
                                         (delete-region ,m-beg ,m-end)
                                         (save-excursion
                                            (goto-char ,m-beg)
                                            (insert final-text)))
                                       (kill-buffer ,orig-buf)
                                       (kill-buffer ,clip-buf)
                                       (set-marker ,m-beg nil)
                                       (set-marker ,m-end nil)
                                       (message "Snippet merged!")))
                                   nil 'local)))))))

(provide 'ggo)
;;; ggo.el ends here
