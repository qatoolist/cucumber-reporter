require "json"
require 'fileutils'
CDIR = ENV['CDIR'] || File.expand_path('../../.counters', __dir__)
FileUtils.mkdir_p(CDIR)
BLOBS = File.expand_path('../../blobs', __dir__)
module Counter
  def self.bump(key)
    f = File.join(CDIR, key.gsub(/[^\w]/, '_'))
    n = File.exist?(f) ? File.read(f).to_i : 0
    File.write(f, (n + 1).to_s)
    n + 1
  end
end
# FLAKY="key:N,key2:N" -> step fails on first N attempts. FAIL="key,key2" -> always fail.
def plan_fail?(key)
  return true if (ENV['FAIL'] || '').split(',').include?(key)
  spec = (ENV['FLAKY'] || '').split(',').map { |s| s.split(':') }.to_h
  return false unless spec[key]
  Counter.bump(key) <= spec[key].to_i
end
module Deep
  def self.level_a(m) = level_b(m)
  def self.level_b(m) = level_c(m)
  def self.level_c(m)
    raise RuntimeError, "#{m}\nline two — ünïcödé ✓ 東京\n\tline three (tab)\nline four <b>&amp;</b>"
  end
end
Before('@skip_on_ci') do
  skip_this_scenario('Skipped on CI: needs a real printer') if ENV['CI'] == 'true'
end
Before do |sc|
  log "browser=#{ENV['BROWSER'] || 'none'} scenario=#{sc.name}"
end
After do |sc|
  if sc.failed?
    attach(File.join(BLOBS, 'shot.png'), 'image/png')
    log "failure context for #{sc.name}: \e[31mred\e[0m"
  end
end
