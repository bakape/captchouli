extern "C" {
#include "thumbnail.h"
}
#include <cstring>
#include <functional>
#include <opencv2/imgproc.hpp>
#include <opencv2/objdetect.hpp>
#include <opencv2/opencv.hpp>
#include <stdexcept>

static const char* _thumbnail(
    cv::CascadeClassifier* c, const Buffer src, Buffer* thumb)
{
    const cv::Mat colour = cv::imdecode(
        cv::Mat(1, src.size, CV_8UC1, src.data), cv::IMREAD_COLOR);
    cv::Mat grayscale, equalized;
    cv::cvtColor(colour, grayscale, cv::COLOR_BGR2GRAY);
    cv::equalizeHist(grayscale, equalized);

    std::vector<cv::Rect> faces;
    c->detectMultiScale(equalized, faces, 1.1, 5, 0, cv::Size(50, 50));
    if (!faces.size()) {
        return "no faces detected";
    }

    cv::Rect face;
    if (faces.size() == 1) {
        face = faces.front();
    } else {
        // Find biggest match
        uint64_t max_size = 0;
        for (auto& f : faces) {
            uint64_t s = (uint64_t)f.width + (uint64_t)f.height;
            if (s > max_size) {
                face = f;
                max_size = s;
            }
        }
    }

    std::vector<unsigned char> out;
    static const std::vector<int> params = { CV_IMWRITE_JPEG_QUALITY, 80 };
    if (!cv::imencode("jpg", cv::Mat(colour, face), out, params)) {
        throw std::domain_error("could not encode result");
    }
    thumb->data = memcpy(malloc(out.size()), out.data(), out.size());
    thumb->size = out.size();
    return 0;
}

static char* malloc_string(const char* s)
{
    return strcpy((char*)malloc(strlen(s) + 1), s);
}

static char* catch_errors(std::function<const char*()> fn)
{
    try {
        auto err = fn();
        return err ? malloc_string(err) : (char*)err;
    } catch (const std::exception& ex) {
        auto err = ex.what();
        return malloc_string(err);
    }
}

extern "C" void* load_classifier(const char* path)
{
    auto c = new cv::CascadeClassifier();
    if (!c->load(path)) {
        delete c;
        return nullptr;
    }
    return c;
}

extern "C" void unload_classifier(void* c)
{
    delete static_cast<cv::CascadeClassifier*>(c);
}

extern "C" char* thumbnail(void* classifier, const Buffer src, Buffer* thumb)
{
    return catch_errors([=]() {
        return _thumbnail(
            static_cast<cv::CascadeClassifier*>(classifier), src, thumb);
    });
}
