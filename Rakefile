# frozen_string_literal: true

require 'time'

task default: %w[push]

task :push do
  system 'rubocop -A'
  system 'git add .'
  system "git commit -m \"Update #{Time.now}\""
  system 'git pull'
  system 'git push origin main'
end

task :deploy do
  system 'docker compose down'
  system 'docker compose up -d --build'
  system 'docker compose logs -f'
end

task :run do
  system 'go run .'
end
