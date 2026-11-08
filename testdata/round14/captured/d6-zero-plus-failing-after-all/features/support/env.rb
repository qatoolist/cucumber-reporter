require 'fileutils'
PLAN = (ENV['PLAN'] || '').split(',').map { |kv| kv.split(':', 2) }.to_h
CDIR = ENV['CDIR'] || 'counters'
FileUtils.mkdir_p(CDIR)
def bump(key)
  f = File.join(CDIR, key.gsub(/[^A-Za-z0-9_]/, '_'))
  n = File.exist?(f) ? File.read(f).to_i : 0
  File.write(f, (n + 1).to_s)
  n + 1
end
