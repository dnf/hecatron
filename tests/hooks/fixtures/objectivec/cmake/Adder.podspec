Pod::Spec.new do |s|
  s.name         = "Adder"
  s.version      = "0.1.0"
  s.summary      = "Adds two numbers."
  s.homepage     = "https://example.com/adder"
  s.license      = { :type => "MIT" }
  s.author       = "Hecatron"
  s.source       = { :git => "https://example.com/adder.git", :tag => s.version.to_s }
  s.source_files = "Adder.{h,m}"
  s.frameworks   = "Foundation"
end
