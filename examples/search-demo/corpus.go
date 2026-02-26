package main

import "time"

type demoDoc struct {
	URL        string
	Title      string
	Body       string
	Outlet     string
	Published  time.Time
	Categories []string
}

var demoCorpus = []demoDoc{
	// AI/ML cluster — terms deliberately overlap across articles for PMI
	{
		URL:   "https://demo.example.com/gpt5-benchmarks",
		Title: "GPT-5 Sets New Benchmarks in Natural Language Processing",
		Body: "OpenAI released GPT-5, a large language model built on an advanced neural network with a new transformer architecture. " +
			"The deep learning model achieves state-of-the-art results on natural language processing benchmarks including MMLU and HumanEval. " +
			"Researchers note significant improvements in the attention mechanism that reduce hallucinations by 40 percent compared to GPT-4.",
		Outlet:     "AI Weekly",
		Published:  time.Now().Add(-2 * 24 * time.Hour),
		Categories: []string{"ai"},
	},
	{
		URL:   "https://demo.example.com/deepmind-protein",
		Title: "DeepMind Uses Deep Learning to Predict New Protein Structures",
		Body: "Google DeepMind announced a breakthrough in computational biology using deep learning models. " +
			"Their neural network can predict protein folding with atomic-level accuracy for previously unknown structures. " +
			"The work builds on AlphaFold and uses reinforcement learning to explore the vast space of possible conformations.",
		Outlet:     "Science Daily",
		Published:  time.Now().Add(-5 * 24 * time.Hour),
		Categories: []string{"ai"},
	},
	{
		URL:   "https://demo.example.com/ml-healthcare",
		Title: "Machine Learning Models Improve Early Cancer Detection",
		Body: "A new study published in Nature Medicine demonstrates that machine learning models using deep learning on medical imaging data " +
			"can detect early-stage cancers with 95 percent accuracy. The neural network approach outperforms traditional " +
			"computer vision methods and could reduce false positive rates in mammography screening by half.",
		Outlet:     "Health Tech Review",
		Published:  time.Now().Add(-3 * 24 * time.Hour),
		Categories: []string{"ai"},
	},
	{
		URL:   "https://demo.example.com/llm-code-generation",
		Title: "Large Language Models Transform Software Engineering Workflows",
		Body: "Enterprise adoption of large language models for code generation has surged this quarter. " +
			"Companies report that artificial intelligence assistants built on neural network architectures reduce boilerplate code by 60 percent " +
			"while maintaining quality. The impact on software engineering productivity is measurable across teams of all sizes.",
		Outlet:     "Dev Weekly",
		Published:  time.Now().Add(-1 * 24 * time.Hour),
		Categories: []string{"ai", "programming"},
	},

	// Security cluster
	{
		URL:   "https://demo.example.com/zero-day-chrome",
		Title: "Critical Zero Day Vulnerability Found in Chrome Browser",
		Body: "Google patched a critical security vulnerability in Chrome that was being actively exploited in the wild. " +
			"The zero day flaw allowed remote code execution through a buffer overflow in the V8 JavaScript engine. " +
			"Security researchers recommend updating immediately as the exploit bypasses existing access control mechanisms.",
		Outlet:     "Security Now",
		Published:  time.Now().Add(-1 * 24 * time.Hour),
		Categories: []string{"security"},
	},
	{
		URL:   "https://demo.example.com/sql-injection-2026",
		Title: "SQL Injection Remains Top Security Vulnerability in 2026",
		Body: "The annual OWASP report confirms that sql injection continues to be the most exploited security vulnerability worldwide. " +
			"Despite decades of awareness, 34 percent of web applications tested had at least one sql injection flaw. " +
			"Cross site scripting ranked second, affecting 28 percent of applications surveyed.",
		Outlet:     "Infosec Magazine",
		Published:  time.Now().Add(-7 * 24 * time.Hour),
		Categories: []string{"security"},
	},
	{
		URL:   "https://demo.example.com/encryption-quantum",
		Title: "Post-Quantum Encryption Algorithm Standardized by NIST",
		Body: "NIST finalized three post-quantum encryption algorithm standards designed to resist attacks from quantum computers. " +
			"The new public key infrastructure replaces RSA and elliptic curve cryptography for government communications. " +
			"Organizations are advised to begin migration planning as certificate authority providers adopt the new standards.",
		Outlet:     "Crypto Weekly",
		Published:  time.Now().Add(-4 * 24 * time.Hour),
		Categories: []string{"security"},
	},

	// DevOps/Infrastructure cluster
	{
		URL:   "https://demo.example.com/k8s-service-mesh",
		Title: "Kubernetes Service Mesh Adoption Reaches 60 Percent",
		Body: "A survey of cloud computing practitioners shows that service mesh adoption in container orchestration environments " +
			"has reached 60 percent. Istio and Linkerd remain the leading distributed system networking solutions. " +
			"Organizations cite improved observability and load balancer configuration as primary benefits.",
		Outlet:     "Cloud Native Times",
		Published:  time.Now().Add(-6 * 24 * time.Hour),
		Categories: []string{"devops"},
	},
	{
		URL:   "https://demo.example.com/serverless-edge",
		Title: "Serverless Architecture Meets Edge Computing",
		Body: "Major cloud providers are combining serverless architecture with edge computing to reduce latency for global applications. " +
			"Infrastructure as code tools now support deploying functions to edge locations with a single configuration change. " +
			"The approach eliminates the need for traditional horizontal scaling strategies in many use cases.",
		Outlet:     "Cloud Architect",
		Published:  time.Now().Add(-2 * 24 * time.Hour),
		Categories: []string{"devops"},
	},
	{
		URL:   "https://demo.example.com/database-migration-guide",
		Title: "Best Practices for Large-Scale Database Migration",
		Body: "A comprehensive guide to database migration strategies for enterprises moving from legacy systems to cloud native platforms. " +
			"The article covers data pipeline design, message queue integration for event driven migration patterns, " +
			"and automated rollback procedures. Batch processing versus streaming data approaches are compared in detail.",
		Outlet:     "DBA Weekly",
		Published:  time.Now().Add(-8 * 24 * time.Hour),
		Categories: []string{"database", "devops"},
	},

	// Open source / Programming cluster
	{
		URL:   "https://demo.example.com/rust-linux-kernel",
		Title: "Rust Language Gains Ground in Linux Kernel Development",
		Body: "The Linux kernel now has over 100 drivers written in Rust, marking a milestone for memory management safety. " +
			"Open source contributors report fewer race condition and buffer overflow bugs compared to equivalent C code. " +
			"The compiler optimization improvements in Rust 2026 edition further reduce the performance gap.",
		Outlet:     "Open Source Weekly",
		Published:  time.Now().Add(-3 * 24 * time.Hour),
		Categories: []string{"programming"},
	},
	{
		URL:   "https://demo.example.com/open-source-funding",
		Title: "Open Source Funding Crisis: Maintainers Speak Out",
		Body: "A coalition of open source maintainers released an open letter highlighting the sustainability crisis in critical software infrastructure. " +
			"Despite software engineering teams worldwide depending on open source libraries, fewer than 5 percent receive any funding. " +
			"The letter calls for corporate sponsors to increase support for package manager ecosystems and build system maintenance.",
		Outlet:     "Dev Community",
		Published:  time.Now().Add(-1 * 24 * time.Hour),
		Categories: []string{"programming"},
	},
	{
		URL:   "https://demo.example.com/functional-programming-revival",
		Title: "Functional Programming Makes a Comeback in Enterprise",
		Body: "Enterprise adoption of functional programming languages has doubled in the past year. " +
			"Scala, Haskell, and F-sharp are seeing increased use in data pipeline and real time processing systems. " +
			"Code review data shows that functional programming styles result in fewer defects per line of code.",
		Outlet:     "Enterprise Dev",
		Published:  time.Now().Add(-5 * 24 * time.Hour),
		Categories: []string{"programming"},
	},

	// Web cluster
	{
		URL:   "https://demo.example.com/wasm-search-engine",
		Title: "WebAssembly Powers Next-Generation Browser Search Engine",
		Body: "A startup launched a privacy-focused search engine that runs entirely in the browser using WebAssembly. " +
			"The progressive web app indexes pages locally, eliminating the need for server side processing. " +
			"User interface responsiveness matches native applications thanks to web component optimizations.",
		Outlet:     "Web Platform Weekly",
		Published:  time.Now().Add(-2 * 24 * time.Hour),
		Categories: []string{"web"},
	},
	{
		URL:   "https://demo.example.com/frontend-framework-survey",
		Title: "Frontend Framework Usage Survey: React Leads, Svelte Surges",
		Body: "The annual web development survey shows React maintaining its lead among frontend framework choices at 62 percent market share. " +
			"Svelte saw the largest growth, jumping from 8 to 18 percent. Responsive design and user experience remain " +
			"the top priorities cited by web development teams across all framework choices.",
		Outlet:     "Frontend Focus",
		Published:  time.Now().Add(-4 * 24 * time.Hour),
		Categories: []string{"web"},
	},

	// Startup cluster
	{
		URL:   "https://demo.example.com/ai-startup-funding",
		Title: "AI Startup Funding Reaches Record Highs in Q1 2026",
		Body: "Venture capital investment in artificial intelligence startups reached 42 billion dollars in Q1 2026. " +
			"Series a rounds averaged 15 million, up from 8 million two years ago. " +
			"Product market fit in enterprise AI is the strongest signal for series b funding according to investors.",
		Outlet:     "Startup Digest",
		Published:  time.Now().Add(-1 * 24 * time.Hour),
		Categories: []string{"startup", "ai"},
	},
	{
		URL:   "https://demo.example.com/saas-business-model",
		Title: "SaaS Business Model Evolution: Usage-Based Pricing Wins",
		Body: "The traditional per-seat SaaS business model is giving way to usage-based pricing. " +
			"Startups with minimum viable product stage products report faster customer acquisition with consumption pricing. " +
			"Revenue stream predictability improves after 12 months as usage patterns stabilize.",
		Outlet:     "SaaS Weekly",
		Published:  time.Now().Add(-6 * 24 * time.Hour),
		Categories: []string{"startup"},
	},

	// Cross-domain articles — bridge clusters for PMI connections
	{
		URL:   "https://demo.example.com/ml-security",
		Title: "Machine Learning Models Detect Security Vulnerabilities in Code",
		Body: "Researchers demonstrated that neural network models trained on code repositories can detect security vulnerability patterns " +
			"with 89 percent accuracy. The deep learning approach identifies potential sql injection and cross site scripting flaws " +
			"before code review. Integration with continuous integration pipelines enables automated scanning.",
		Outlet:     "AI Security Review",
		Published:  time.Now().Add(-2 * 24 * time.Hour),
		Categories: []string{"ai", "security"},
	},
	{
		URL:   "https://demo.example.com/cloud-ai-inference",
		Title: "Cloud Computing Giants Race to Offer AI Inference at Scale",
		Body: "AWS, Google Cloud, and Azure are competing to provide the most cost-effective infrastructure for large language model inference. " +
			"Cloud computing costs for deep learning and neural network inference have dropped 70 percent in the past year. " +
			"Container orchestration platforms now include dedicated machine learning workload scheduling optimized for GPU utilization.",
		Outlet:     "Cloud Report",
		Published:  time.Now().Add(-3 * 24 * time.Hour),
		Categories: []string{"devops", "ai"},
	},
	{
		URL:   "https://demo.example.com/devtools-ai",
		Title: "AI-Powered Developer Tools Reshape Software Architecture",
		Body: "A new generation of integrated development environment plugins uses artificial intelligence and machine learning to suggest software architecture improvements. " +
			"The tools analyze design pattern usage across the codebase and recommend refactoring opportunities. " +
			"Early adopters report a 30 percent reduction in technical debt when following AI-guided code review suggestions.",
		Outlet:     "DevTools Digest",
		Published:  time.Now().Add(-4 * 24 * time.Hour),
		Categories: []string{"programming", "ai"},
	},
}
