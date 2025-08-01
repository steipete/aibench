#!/usr/bin/env python3
"""
AI Model Benchmark Configuration
Compares Cerebras qwen-3-coder-480b vs Alibaba Cloud qwen3-coder-plus
"""

import json
import time
import asyncio
import aiohttp
from typing import Dict, List, Any, Optional
from dataclasses import dataclass
from datetime import datetime

@dataclass
class BenchmarkConfig:
    """Configuration for benchmark test"""
    name: str
    description: str
    prompt: str
    expected_tokens: int = 500
    max_tokens: int = 1000
    temperature: float = 0.7

@dataclass
class ModelEndpoint:
    """Model endpoint configuration"""
    name: str
    base_url: str
    api_key: str
    model_name: str
    headers: Dict[str, str]

# Model configurations
CEREBRAS_ENDPOINT = ModelEndpoint(
    name="Cerebras",
    base_url="https://api.cerebras.ai/v1",
    api_key="csk-93edwytptjcv38v3yvpvhtxxwch3y2yjrffe6h89nv5hr26m",
    model_name="qwen-3-coder-480b",
    headers={
        "Content-Type": "application/json",
        "Authorization": "Bearer csk-93edwytptjcv38v3yvpvhtxxwch3y2yjrffe6h89nv5hr26m"
    }
)

ALIBABA_ENDPOINT = ModelEndpoint(
    name="Alibaba Cloud",
    base_url="https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
    api_key="sk-92d6819e0f2442dc86c71a33002a1d1e",
    model_name="qwen3-coder-plus",
    headers={
        "Content-Type": "application/json",
        "Authorization": "Bearer sk-92d6819e0f2442dc86c71a33002a1d1e"
    }
)

CUSTOM_ENDPOINT = ModelEndpoint(
    name="Custom Server",
    base_url="http://86.38.182.74:8000",
    api_key="none",  # Assuming no auth required for custom server
    model_name="qwen3-coder",
    headers={
        "Content-Type": "application/json"
    }
)

# Test prompts covering different coding scenarios
BENCHMARK_TESTS = [
    BenchmarkConfig(
        name="simple_function",
        description="Simple Python function implementation",
        prompt="Write a Python function that takes a list of integers and returns the sum of even numbers.",
        expected_tokens=200,
        max_tokens=500
    ),
    BenchmarkConfig(
        name="algorithm_implementation",
        description="Algorithm implementation with explanation",
        prompt="Implement a binary search algorithm in Python with detailed comments explaining each step. Include time and space complexity analysis.",
        expected_tokens=400,
        max_tokens=800
    ),
    BenchmarkConfig(
        name="data_structure",
        description="Data structure implementation",
        prompt="Create a Python class for a binary search tree with insert, delete, and search methods. Include proper error handling.",
        expected_tokens=600,
        max_tokens=1200
    ),
    BenchmarkConfig(
        name="debugging_task",
        description="Code debugging and fixing",
        prompt="""Debug this Python code and fix all issues:

def quicksort(arr):
    if len(arr) <= 1:
        return arr
    pivot = arr[len(arr) // 2]
    left = [x for x in arr if x < pivot]
    middle = [x for x in arr if x = pivot]
    right = [x for x in arr if x > pivot]
    return quicksort(left) + middle + quicksort(right)

# Test
numbers = [3, 6, 8, 10, 1, 2, 1]
print(quicksort(numbers))""",
        expected_tokens=300,
        max_tokens=600
    ),
    BenchmarkConfig(
        name="code_explanation",
        description="Code explanation and optimization",
        prompt="""Explain this JavaScript code and suggest optimizations:

function fibonacci(n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}

for (let i = 0; i < 40; i++) {
    console.log(`fib(${i}) = ${fibonacci(i)}`);
}""",
        expected_tokens=400,
        max_tokens=800
    ),
    BenchmarkConfig(
        name="api_design",
        description="REST API design task",
        prompt="Design a RESTful API for a todo list application. Include endpoints, HTTP methods, request/response formats, and error handling. Provide example code in Python using Flask.",
        expected_tokens=700,
        max_tokens=1400
    ),
    BenchmarkConfig(
        name="performance_analysis",
        description="Performance analysis and profiling",
        prompt="Analyze the performance bottlenecks in this Python code and provide optimized versions with explanations:\n\ndef process_data(data):\n    result = []\n    for item in data:\n        if item % 2 == 0:\n            result.append(item * 2)\n    return sorted(result, reverse=True)",
        expected_tokens=500,
        max_tokens=1000
    ),
    BenchmarkConfig(
        name="complex_system_design",
        description="System design question",
        prompt="Design a distributed caching system similar to Redis. Explain the architecture, data structures, consistency models, and provide pseudocode for key operations (GET, SET, DELETE). Consider scalability and fault tolerance.",
        expected_tokens=800,
        max_tokens=1600
    )
]

@dataclass
class BenchmarkResult:
    """Results from a single benchmark test"""
    test_name: str
    model_name: str
    response: str
    response_time: float
    tokens_generated: int
    tokens_per_second: float
    success: bool
    error_message: Optional[str] = None
    timestamp: str = ""

class ModelBenchmark:
    """Benchmark runner for comparing AI models"""
    
    def __init__(self):
        self.results: List[BenchmarkResult] = []
    
    async def call_model(self, endpoint: ModelEndpoint, config: BenchmarkConfig) -> BenchmarkResult:
        """Call a model endpoint and measure performance"""
        start_time = time.time()
        
        payload = {
            "model": endpoint.model_name,
            "messages": [
                {"role": "user", "content": config.prompt}
            ],
            "max_tokens": config.max_tokens,
            "temperature": config.temperature,
            "stream": False
        }
        
        try:
            # Handle custom server without auth
            headers = endpoint.headers.copy()
            if endpoint.api_key != "none":
                headers["Authorization"] = f"Bearer {endpoint.api_key}"
            
            async with aiohttp.ClientSession() as session:
                async with session.post(
                    f"{endpoint.base_url}/chat/completions",
                    headers=headers,
                    json=payload,
                    timeout=aiohttp.ClientTimeout(total=60)
                ) as response:
                    response_time = time.time() - start_time
                    
                    if response.status != 200:
                        error_text = await response.text()
                        return BenchmarkResult(
                            test_name=config.name,
                            model_name=endpoint.name,
                            response="",
                            response_time=response_time,
                            tokens_generated=0,
                            tokens_per_second=0,
                            success=False,
                            error_message=f"HTTP {response.status}: {error_text}",
                            timestamp=datetime.now().isoformat()
                        )
                    
                    data = await response.json()
                    
                    if "choices" not in data or not data["choices"]:
                        return BenchmarkResult(
                            test_name=config.name,
                            model_name=endpoint.name,
                            response="",
                            response_time=response_time,
                            tokens_generated=0,
                            tokens_per_second=0,
                            success=False,
                            error_message="No response choices returned",
                            timestamp=datetime.now().isoformat()
                        )
                    
                    response_text = data["choices"][0]["message"]["content"]
                    
                    # Estimate tokens (rough approximation: 1 token ≈ 4 characters)
                    tokens_generated = len(response_text) // 4
                    tokens_per_second = tokens_generated / response_time if response_time > 0 else 0
                    
                    return BenchmarkResult(
                        test_name=config.name,
                        model_name=endpoint.name,
                        response=response_text,
                        response_time=response_time,
                        tokens_generated=tokens_generated,
                        tokens_per_second=tokens_per_second,
                        success=True,
                        timestamp=datetime.now().isoformat()
                    )
        
        except Exception as e:
            response_time = time.time() - start_time
            return BenchmarkResult(
                test_name=config.name,
                model_name=endpoint.name,
                response="",
                response_time=response_time,
                tokens_generated=0,
                tokens_per_second=0,
                success=False,
                error_message=str(e),
                timestamp=datetime.now().isoformat()
            )
    
    async def run_benchmark(self, test_config: BenchmarkConfig) -> List[BenchmarkResult]:
        """Run a single benchmark test against all models"""
        print(f"Running test: {test_config.name}")
        
        # Run all models concurrently
        tasks = [
            self.call_model(CEREBRAS_ENDPOINT, test_config),
            self.call_model(ALIBABA_ENDPOINT, test_config),
            self.call_model(CUSTOM_ENDPOINT, test_config)
        ]
        
        results = await asyncio.gather(*tasks, return_exceptions=True)
        
        valid_results = []
        for result in results:
            if isinstance(result, Exception):
                print(f"Error in benchmark: {result}")
            else:
                valid_results.append(result)
                self.results.append(result)
        
        return valid_results
    
    async def run_all_benchmarks(self) -> None:
        """Run all benchmark tests"""
        print("Starting AI Model Benchmark")
        print("=" * 50)
        
        for test_config in BENCHMARK_TESTS:
            results = await self.run_benchmark(test_config)
            
            # Display immediate results
            for result in results:
                status = "✓" if result.success else "✗"
                print(f"{status} {result.model_name}: {result.response_time:.2f}s, "
                      f"{result.tokens_per_second:.1f} tokens/s")
            
            print("-" * 30)
            # Small delay between tests to be respectful to APIs
            await asyncio.sleep(1)
    
    def save_results(self, filename: str = "benchmark_results.json") -> None:
        """Save benchmark results to JSON file"""
        results_data = []
        for result in self.results:
            results_data.append({
                "test_name": result.test_name,
                "model_name": result.model_name,
                "response_time": result.response_time,
                "tokens_generated": result.tokens_generated,
                "tokens_per_second": result.tokens_per_second,
                "success": result.success,
                "error_message": result.error_message,
                "timestamp": result.timestamp,
                "response_preview": result.response[:200] + "..." if len(result.response) > 200 else result.response
            })
        
        with open(filename, 'w') as f:
            json.dump(results_data, f, indent=2)
        
        print(f"Results saved to {filename}")
    
    def print_summary(self) -> None:
        """Print benchmark summary"""
        print("\n" + "=" * 50)
        print("BENCHMARK SUMMARY")
        print("=" * 50)
        
        # Group results by model
        model_results = {}
        for result in self.results:
            if result.model_name not in model_results:
                model_results[result.model_name] = []
            model_results[result.model_name].append(result)
        
        for model_name, results in model_results.items():
            successful_results = [r for r in results if r.success]
            failed_results = [r for r in results if not r.success]
            
            print(f"\n{model_name}:")
            print(f"  Successful tests: {len(successful_results)}/{len(results)}")
            
            if successful_results:
                avg_response_time = sum(r.response_time for r in successful_results) / len(successful_results)
                avg_tokens_per_sec = sum(r.tokens_per_second for r in successful_results) / len(successful_results)
                
                print(f"  Average response time: {avg_response_time:.2f}s")
                print(f"  Average tokens/second: {avg_tokens_per_sec:.1f}")
            
            if failed_results:
                print(f"  Failed tests: {len(failed_results)}")
                for failed in failed_results:
                    print(f"    - {failed.test_name}: {failed.error_message}")

if __name__ == "__main__":
    async def main():
        benchmark = ModelBenchmark()
        await benchmark.run_all_benchmarks()
        benchmark.save_results()
        benchmark.print_summary()
    
    asyncio.run(main())